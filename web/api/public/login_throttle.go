package public

// Login throttling for the unauthenticated POST /api/login endpoint.
//
// Two independent, memory-bounded limiters:
//
//   - Source limiter: a token bucket keyed by the TCP direct peer (the host
//     part of Request.RemoteAddr). The application never configures gin
//     TrustedProxies, so c.ClientIP() honors spoofable X-Forwarded-For headers
//     and must not be used for enforcement. The accepted trade-off: clients
//     behind the same reverse proxy share one source bucket (documented in
//     SECURITY.md and the PR description).
//   - Account limiter: a short-lived exponential cooldown keyed by
//     sha256(raw submitted username). It counts every attempt that does not
//     end in a created session (wrong password, missing or wrong 2FA) and is
//     reset only after a fully successful login, so an attacker holding the
//     password cannot spam wrong 2FA codes without accumulating failures.
//     There is no permanent lockout: cooldowns cap at 60s and the whole state
//     resets after the inactivity TTL.
//
// Both limiters follow the bounded-map pattern of the visitor audit rate
// limiter (mutex + entry cap + TTL cleanup), but eviction frees capacity by
// dropping stale and then least-recently-seen entries instead of denying new
// ones, so flooding the map can never lock out unrelated logins.

import (
	"crypto/sha256"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/komari-monitor/komari/web/api"

	"github.com/gin-gonic/gin"
)

// Source bucket: burst 10 requests, then a sustained 30 requests/minute per
// direct peer. Komari deployments have few administrators, and a full 2FA
// login costs two requests (the first reply asks for the 2FA code), so a
// single human login stays far below the burst; password-manager retries and
// double submits fit inside the burst. Behind one reverse proxy or NAT the
// whole client population shares one bucket, hence the deliberately generous
// sustained rate: 30/min covers a small team logging in simultaneously while
// still bounding per-source KDF work for the future Argon2id migration
// (PR3) to at most 30 verifications per minute per source.
const (
	loginSourceBurst        = 10
	loginSourceRatePerMin   = 30
	loginSourceTTL          = 10 * time.Minute
	loginSourceMaxEntries   = 10000
	loginSourceRetryCeiling = 1 * time.Minute
)

// Account bucket: the first failures are answered normally (the user may
// just typo a password); from the threshold on, each further failure extends
// an exponential cooldown. The schedule is intentionally short (max 60s) so
// an attacker cannot weaponize the limiter into a durable denial-of-service
// against the administrator. Fifteen minutes without any failure restores
// the account to a clean state.
const (
	loginAccountThreshold  = 5
	loginAccountMaxEntries = 20000
	loginAccountTTL        = 15 * time.Minute
)

var loginAccountCooldowns = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	20 * time.Second,
	40 * time.Second,
	60 * time.Second,
}

type loginSourceState struct {
	tokens     float64
	lastRefill time.Time
	lastSeen   time.Time
}

type loginSourceLimiter struct {
	mu      sync.Mutex
	entries map[string]*loginSourceState
}

var loginSources = newLoginSourceLimiter()

func newLoginSourceLimiter() *loginSourceLimiter {
	return &loginSourceLimiter{entries: make(map[string]*loginSourceState)}
}

// allow consumes one token for the source. When denied it reports how long
// until the next token, capped at a conservative ceiling.
func (l *loginSourceLimiter) allow(source string, now time.Time) (ok bool, wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state, exists := l.entries[source]
	if !exists {
		l.evictLocked(now)
		l.entries[source] = &loginSourceState{
			tokens:     loginSourceBurst - 1,
			lastRefill: now,
			lastSeen:   now,
		}
		return true, 0
	}
	if elapsed := now.Sub(state.lastRefill); elapsed > 0 {
		state.tokens = min(float64(loginSourceBurst),
			state.tokens+elapsed.Minutes()*loginSourceRatePerMin)
		state.lastRefill = now
	}
	state.lastSeen = now
	if state.tokens < 1 {
		wait = time.Duration((1 - state.tokens) / loginSourceRatePerMin * float64(time.Minute))
		if wait > loginSourceRetryCeiling {
			wait = loginSourceRetryCeiling
		}
		return false, wait
	}
	state.tokens--
	return true, 0
}

// evictLocked keeps the map bounded: stale entries go first; if that is not
// enough, the least recently seen entry is dropped to free one slot. New
// sources are never denied just because the map is full.
func (l *loginSourceLimiter) evictLocked(now time.Time) {
	if len(l.entries) < loginSourceMaxEntries {
		return
	}
	for key, state := range l.entries {
		if now.Sub(state.lastSeen) >= loginSourceTTL {
			delete(l.entries, key)
		}
	}
	for len(l.entries) >= loginSourceMaxEntries {
		oldestKey := ""
		oldest := time.Time{}
		for key, state := range l.entries {
			if oldest.IsZero() || state.lastSeen.Before(oldest) {
				oldestKey, oldest = key, state.lastSeen
			}
		}
		delete(l.entries, oldestKey)
	}
}

type loginAccountState struct {
	failures      int
	cooldownUntil time.Time
	lastSeen      time.Time
}

type loginAccountLimiter struct {
	mu      sync.Mutex
	entries map[[sha256.Size]byte]*loginAccountState
}

var loginAccounts = newLoginAccountLimiter()

func newLoginAccountLimiter() *loginAccountLimiter {
	return &loginAccountLimiter{entries: make(map[[sha256.Size]byte]*loginAccountState)}
}

// loginAccountKey hashes the raw submitted username. The hash is used purely
// as a fixed-size map key: the exact bytes are kept (no trimming or case
// folding) because the login query matches usernames verbatim, and hashing
// keeps attacker-controlled strings of arbitrary length out of long-lived
// map entries without revealing which usernames exist.
func loginAccountKey(username string) [sha256.Size]byte {
	return sha256.Sum256([]byte(username))
}

// check reports whether an attempt for the account may proceed and, while a
// cooldown is active, how much of it remains. It never counts as activity:
// only failures refresh the state.
func (l *loginAccountLimiter) check(key [sha256.Size]byte, now time.Time) (ok bool, wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state, exists := l.entries[key]
	if !exists {
		return true, 0
	}
	if now.Sub(state.lastSeen) >= loginAccountTTL {
		delete(l.entries, key)
		return true, 0
	}
	if now.Before(state.cooldownUntil) {
		return false, state.cooldownUntil.Sub(now)
	}
	return true, 0
}

// recordFailure counts an attempt that did not create a session (wrong
// password, or missing/wrong 2FA after a correct password).
func (l *loginAccountLimiter) recordFailure(key [sha256.Size]byte, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	state, exists := l.entries[key]
	if !exists {
		l.evictLocked(now)
		state = &loginAccountState{}
		l.entries[key] = state
	} else if now.Sub(state.lastSeen) >= loginAccountTTL {
		state.failures = 0
		state.cooldownUntil = time.Time{}
	}
	state.failures++
	state.lastSeen = now
	if state.failures >= loginAccountThreshold {
		index := state.failures - loginAccountThreshold
		if index >= len(loginAccountCooldowns) {
			index = len(loginAccountCooldowns) - 1
		}
		state.cooldownUntil = now.Add(loginAccountCooldowns[index])
	}
}

// reset clears the failure state of an account after a fully successful
// login (password + 2FA + session).
func (l *loginAccountLimiter) reset(key [sha256.Size]byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *loginAccountLimiter) evictLocked(now time.Time) {
	if len(l.entries) < loginAccountMaxEntries {
		return
	}
	for key, state := range l.entries {
		if now.Sub(state.lastSeen) >= loginAccountTTL {
			delete(l.entries, key)
		}
	}
	for len(l.entries) >= loginAccountMaxEntries {
		var oldestKey [sha256.Size]byte
		oldest := time.Time{}
		for key, state := range l.entries {
			if oldest.IsZero() || state.lastSeen.Before(oldest) {
				oldestKey, oldest = key, state.lastSeen
			}
		}
		delete(l.entries, oldestKey)
	}
}

// loginEnforcementSource returns the host portion of the TCP direct peer.
// RemoteAddr cannot be forged by request headers, unlike the forwarded
// headers honored by c.ClientIP() when no trusted proxy chain is configured.
func loginEnforcementSource(c *gin.Context) string {
	if host, _, err := net.SplitHostPort(c.Request.RemoteAddr); err == nil && host != "" {
		return host
	}
	return c.Request.RemoteAddr
}

// respondLoginThrottled replies with the uniform 429 body. The message never
// reveals whether the source or the account bucket denied the request nor
// whether the username exists. Retry-After is set only when a wait was
// reliably computed and is clamped to at least one second.
func respondLoginThrottled(c *gin.Context, wait time.Duration) {
	if wait > 0 {
		seconds := int(math.Ceil(wait.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		if ceiling := int(loginSourceRetryCeiling.Seconds()); seconds > ceiling {
			seconds = ceiling
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
	}
	api.RespondError(c, http.StatusTooManyRequests, "Too many login attempts")
}
