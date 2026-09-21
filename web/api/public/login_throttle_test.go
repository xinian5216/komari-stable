package public

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"

	"github.com/gin-gonic/gin"
)

// resetLoginLimiters swaps in fresh limiter instances so handler tests are
// deterministic and independent of each other.
func resetLoginLimiters(t *testing.T) {
	t.Helper()
	originalSource, originalAccount := loginSources, loginAccounts
	loginSources = newLoginSourceLimiter()
	loginAccounts = newLoginAccountLimiter()
	t.Cleanup(func() {
		loginSources, loginAccounts = originalSource, originalAccount
	})
}

func newLoginTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", Login)
	return router
}

func performLogin(router *gin.Engine, remoteAddr, username, password, twoFa string, forwardedFor string) *httptest.ResponseRecorder {
	body := map[string]string{}
	if username != "" {
		body["username"] = username
	}
	if password != "" {
		body["password"] = password
	}
	if twoFa != "" {
		body["2fa_code"] = twoFa
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Source limiter (unit) ---

func TestLoginSourceLimiterBurstThenThrottle(t *testing.T) {
	limiter := newLoginSourceLimiter()
	now := time.Now()

	// The burst is per source, regardless of the submitted username.
	for i := 0; i < loginSourceBurst; i++ {
		ok, wait := limiter.allow("203.0.113.7", now)
		assert.True(t, ok, "request %d within burst should pass", i+1)
		assert.Zero(t, wait)
	}
	ok, wait := limiter.allow("203.0.113.7", now)
	assert.False(t, ok, "request beyond burst must be throttled")
	assert.True(t, wait > 0, "denied source request must report a positive wait")

	// Refill: one token per 2 seconds at 30/min.
	ok, _ = limiter.allow("203.0.113.7", now.Add(2*time.Second+time.Millisecond))
	assert.True(t, ok, "source must recover after refill time passed")

	// A different source has its own bucket.
	ok, _ = limiter.allow("203.0.113.8", now)
	assert.True(t, ok, "a different source must not inherit the throttle")
}

func TestLoginSourceLimiterCapacityBound(t *testing.T) {
	limiter := newLoginSourceLimiter()
	now := time.Now()
	for i := 0; i < loginSourceMaxEntries+5000; i++ {
		limiter.allow("10.255."+strconv.Itoa(i/255)+"."+strconv.Itoa(i%255), now)
	}
	limiter.mu.Lock()
	size := len(limiter.entries)
	limiter.mu.Unlock()
	assert.LessOrEqual(t, size, loginSourceMaxEntries,
		"flooding the source map must never exceed the entry cap")
}

func TestLoginSourceLimiterEvictsStaleBeforeOldest(t *testing.T) {
	limiter := newLoginSourceLimiter()
	now := time.Now()
	for i := 0; i < loginSourceMaxEntries; i++ {
		limiter.allow("192.0.2."+strconv.Itoa(i%255)+"."+strconv.Itoa(i), now)
	}
	// Every entry is stale relative to a much later "now": capacity pressure
	// must reclaim them rather than deny the new source.
	ok, _ := limiter.allow("198.51.100.9", now.Add(loginSourceTTL+time.Minute))
	assert.True(t, ok, "stale map content must not block a new source")
}

// --- Account limiter (unit) ---

func TestLoginAccountLimiterCooldownScheduleAndExpiry(t *testing.T) {
	limiter := newLoginAccountLimiter()
	key := loginAccountKey("admin")
	now := time.Now()

	// First failures are answered normally.
	for i := 1; i < loginAccountThreshold; i++ {
		limiter.recordFailure(key, now)
		ok, wait := limiter.check(key, now)
		assert.True(t, ok, "failure %d below threshold must not cool down", i)
		assert.Zero(t, wait)
	}
	// The threshold failure starts the shortest cooldown.
	limiter.recordFailure(key, now)
	ok, wait := limiter.check(key, now)
	assert.False(t, ok)
	assert.Equal(t, loginAccountCooldowns[0], wait)

	// Cooldowns escalate but are capped: simulate many consecutive failures.
	for i := 0; i < 50; i++ {
		limiter.recordFailure(key, now.Add(time.Duration(i)*time.Minute))
	}
	// The last failure happened 30 seconds ago in wall time and set a
	// maximum (60s) cooldown, so half a minute of it remains.
	ok, wait = limiter.check(key, now.Add(49*time.Minute+30*time.Second))
	assert.False(t, ok)
	assert.Equal(t, 30*time.Second, wait,
		"cooldown must cap at the schedule maximum")

	// After the cooldown expires a new attempt is allowed again (no
	// permanent lockout), and after the inactivity TTL the state resets.
	ok, _ = limiter.check(key, now.Add(50*time.Minute+loginAccountCooldowns[len(loginAccountCooldowns)-1]+time.Second))
	assert.True(t, ok, "an expired cooldown must allow a new attempt")

	ok, _ = limiter.check(key, now.Add(24*time.Hour))
	assert.True(t, ok, "TTL must restore a clean state")
	limiter.mu.Lock()
	_, exists := limiter.entries[key]
	limiter.mu.Unlock()
	assert.False(t, exists, "stale account state must be dropped")
}

func TestLoginAccountLimiterAccumulatesAcrossSources(t *testing.T) {
	limiter := newLoginAccountLimiter()
	key := loginAccountKey("admin")
	now := time.Now()
	// The account bucket is keyed by username only, so rotating the source
	// cannot dodge it.
	for i := 0; i < loginAccountThreshold; i++ {
		limiter.recordFailure(key, now)
	}
	ok, _ := limiter.check(key, now)
	assert.False(t, ok, "account failures must accumulate regardless of source")
}

func TestLoginAccountLimiterKeyIsRawUsername(t *testing.T) {
	// Matching semantics of the login query are preserved verbatim: no
	// trimming, no case folding.
	assert.NotEqual(t, loginAccountKey("admin"), loginAccountKey("Admin"))
	assert.NotEqual(t, loginAccountKey("admin"), loginAccountKey(" admin"))
}

func TestLoginAccountLimiterCapacityBound(t *testing.T) {
	limiter := newLoginAccountLimiter()
	now := time.Now()
	for i := 0; i < loginAccountMaxEntries+5000; i++ {
		limiter.recordFailure(loginAccountKey(fmt.Sprintf("attacker-%d", i)), now)
	}
	limiter.mu.Lock()
	size := len(limiter.entries)
	limiter.mu.Unlock()
	assert.LessOrEqual(t, size, loginAccountMaxEntries,
		"mass random usernames must not grow the account map without bound")
}

func TestLoginLimitersConcurrent(t *testing.T) {
	sources := newLoginSourceLimiter()
	accountsLimiter := newLoginAccountLimiter()
	base := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for round := 0; round < 200; round++ {
				now := base.Add(time.Duration(round) * 3 * time.Millisecond)
				sources.allow(fmt.Sprintf("10.%d.%d.1", worker, round%7), now)
				key := loginAccountKey(fmt.Sprintf("user-%d", round%11))
				accountsLimiter.check(key, now)
				accountsLimiter.recordFailure(key, now)
				accountsLimiter.reset(key)
			}
		}(worker)
	}
	wg.Wait()

	sources.mu.Lock()
	sourceSize := len(sources.entries)
	sources.mu.Unlock()
	accountsLimiter.mu.Lock()
	accountSize := len(accountsLimiter.entries)
	accountsLimiter.mu.Unlock()
	assert.LessOrEqual(t, sourceSize, loginSourceMaxEntries)
	assert.LessOrEqual(t, accountSize, loginAccountMaxEntries)
}

// --- 429 response shape ---

func TestRespondLoginThrottledHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		wait       time.Duration
		wantHeader string
	}{
		{"no wait", 0, ""},
		{"sub-second wait", 300 * time.Millisecond, "1"},
		{"normal wait", 5 * time.Second, "5"},
		{"capped wait", loginSourceRetryCeiling * 10, strconv.Itoa(int(loginSourceRetryCeiling.Seconds()))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			respondLoginThrottled(c, tt.wait)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, tt.wantHeader, w.Header().Get("Retry-After"))
			var body map[string]any
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, "error", body["status"])
			assert.Equal(t, "Too many login attempts", body["message"])
			// The body must not leak the triggering bucket or username.
			assert.NotContains(t, body, "source")
			assert.NotContains(t, body, "account")
			assert.Nil(t, body["data"])
		})
	}
}

// --- Handler integration ---

func TestLoginSourceThrottleSamePeerDifferentUsernames(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	for i := 0; i < loginSourceBurst; i++ {
		w := performLogin(router, "203.0.113.10:1000", fmt.Sprintf("user-%d", i), "whatever", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code, "username %d", i)
	}
	w := performLogin(router, "203.0.113.10:1000", "another-user", "whatever", "", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"switching usernames must not bypass the source limiter")
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	assert.NoError(t, err, "Retry-After must be a plain integer when throttled")
	assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After must never be negative or zero")
}

func TestLoginAccountThrottleAcrossSourceIPs(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	for i := 0; i < loginAccountThreshold; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 20+i)
		w := performLogin(router, remote, "admin", "wrong-password", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code, "failure %d", i+1)
	}
	// A brand new source is still denied for the cooled-down account.
	w := performLogin(router, "198.51.100.99:1000", "admin", "wrong-password", "", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"rotating the source must not allow endless brute force on one account")
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestLoginThrottleUnknownUsernameIsTracked(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	for i := 0; i < loginAccountThreshold; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 40+i)
		w := performLogin(router, remote, "does-not-exist", "wrong-password", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	w := performLogin(router, "198.51.100.77:1000", "does-not-exist", "wrong-password", "", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"nonexistent usernames must be tracked the same way")
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Too many login attempts", body["message"],
		"the 429 body must not reveal whether the username exists")
}

func TestLoginSuccessClearsAccountState(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	_, err := accounts.CreateAccount("throttle-success", "correct-horse-battery")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = accounts.DeleteAccountByUsername("throttle-success") })
	key := loginAccountKey("throttle-success")

	for i := 0; i < loginAccountThreshold-1; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 60+i)
		w := performLogin(router, remote, "throttle-success", "wrong-password", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}

	// Full success: password + session.
	w := performLogin(router, "203.0.113.70:1000", "throttle-success", "correct-horse-battery", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	loginAccounts.mu.Lock()
	_, exists := loginAccounts.entries[key]
	loginAccounts.mu.Unlock()
	assert.False(t, exists, "a fully successful login must clear the failure state")

	// The counter restarts from zero: the same number of fresh failures is
	// needed before a cooldown appears again.
	for i := 0; i < loginAccountThreshold-1; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 80+i)
		w := performLogin(router, remote, "throttle-success", "wrong-password", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code, "post-success failure %d", i+1)
	}
	w = performLogin(router, "203.0.113.90:1000", "throttle-success", "wrong-password", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"failures just below the threshold after a success must still be answered normally")
}

func TestLogin2FAFailuresAccumulateUntilFullSuccess(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	user, err := accounts.CreateAccount("throttle-2fa", "correct-horse-battery")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = accounts.DeleteAccountByUsername("throttle-2fa") })

	secret, _, err := accounts.Generate2Fa()
	assert.NoError(t, err)
	assert.NoError(t, accounts.Enable2Fa(user.UUID, secret))

	// Correct password but missing 2FA: five such requests must accumulate
	// account failures (no reset after the password check).
	for i := 0; i < loginAccountThreshold; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 100+i)
		w := performLogin(router, remote, "throttle-2fa", "correct-horse-battery", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var body map[string]any
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "2FA code is required", body["message"])
	}
	w := performLogin(router, "203.0.113.110:1000", "throttle-2fa", "correct-horse-battery", "", "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"correct password with missing 2FA must not reset the account limiter")

	// Wrong 2FA code counts as well.
	resetLoginLimiters(t)
	badCode := "123456"
	if validCode, err := totp.GenerateCode(secret, time.Now()); err == nil && validCode == badCode {
		badCode = "654321"
	}
	for i := 0; i < loginAccountThreshold; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1000", 120+i)
		w := performLogin(router, remote, "throttle-2fa", "correct-horse-battery", badCode, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	w = performLogin(router, "203.0.113.130:1000", "throttle-2fa", "correct-horse-battery", badCode, "")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"wrong 2FA codes must accumulate account failures")

	// Full success with a valid TOTP clears the state.
	resetLoginLimiters(t)
	code, err := totp.GenerateCode(secret, time.Now())
	assert.NoError(t, err)
	w = performLogin(router, "203.0.113.140:1000", "throttle-2fa", "correct-horse-battery", code, "")
	assert.Equal(t, http.StatusOK, w.Code)
	loginAccounts.mu.Lock()
	_, exists := loginAccounts.entries[loginAccountKey("throttle-2fa")]
	loginAccounts.mu.Unlock()
	assert.False(t, exists, "full 2FA success must clear the failure state")
}

func TestLoginThrottleIgnoresSpoofedForwardedFor(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	for i := 0; i < loginSourceBurst; i++ {
		w := performLogin(router, "203.0.113.150:1000", "admin", "whatever", "",
			fmt.Sprintf("192.0.2.%d", 200+i))
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	w := performLogin(router, "203.0.113.150:1000", "admin", "whatever", "", "192.0.2.250")
	assert.Equal(t, http.StatusTooManyRequests, w.Code,
		"spoofed X-Forwarded-For must not bypass the source limiter")
}

func TestLoginPasswordDisabledKeepsForbidden(t *testing.T) {
	resetLoginLimiters(t)
	router := newLoginTestRouter()

	// Exhaust the source bucket first.
	for i := 0; i < loginSourceBurst; i++ {
		performLogin(router, "203.0.113.160:1000", "admin", "whatever", "", "")
	}

	assert.NoError(t, config.Set(config.DisablePasswordLoginKey, true))
	t.Cleanup(func() { _ = config.Set(config.DisablePasswordLoginKey, false) })

	w := performLogin(router, "203.0.113.160:1000", "admin", "whatever", "", "")
	assert.Equal(t, http.StatusForbidden, w.Code,
		"the disabled-login 403 must win over the throttle 429")
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Password login is disabled", body["message"])
}
