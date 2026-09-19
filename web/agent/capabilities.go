package agent

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

// Capability names as reported by agents. Remote-control names remain only as
// protocol compatibility sentinels and are never accepted or advertised.
const (
	CapabilityPing     = "ping"
	CapabilityMessage  = "message"
	CapabilityEvent    = "event"
	CapabilityExec     = "exec"
	CapabilityTerminal = "terminal"
	CapabilityFile     = "file"
)

// ErrCapabilityUnavailable is returned for protocol methods whose remote-control
// implementation has been removed from Komari Stable.
var ErrCapabilityUnavailable = errors.New("capability unavailable")

type reportedCapabilities struct {
	capabilities   []string
	privilegeLevel string
	// reported is false for agents that never told the server what they can do.
	// It is retained for protocol compatibility and monitoring capability data;
	// it never enables remote control.
	reported bool
}

var (
	capabilityMu    sync.RWMutex
	capabilityStore = make(map[string]reportedCapabilities)
)

// SetClientCapabilities records what an agent reported about itself. An empty
// privilege level (the POST fallback only carries capabilities) keeps the
// previously reported privilege level.
func SetClientCapabilities(uuid string, capabilities []string, privilegeLevel string) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" || capabilities == nil {
		return
	}

	cleaned := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability = strings.TrimSpace(capability); capability != "" && !isRemoteControlCapability(capability) {
			cleaned = append(cleaned, capability)
		}
	}

	capabilityMu.Lock()
	defer capabilityMu.Unlock()

	privilegeLevel = strings.TrimSpace(privilegeLevel)
	current := capabilityStore[uuid]
	if privilegeLevel == "" {
		privilegeLevel = current.privilegeLevel
	}
	capabilityStore[uuid] = reportedCapabilities{
		capabilities:   cleaned,
		privilegeLevel: privilegeLevel,
		reported:       true,
	}
}

// ClientCapabilities returns the capabilities an agent reported. reported is
// false when nothing was reported.
func ClientCapabilities(uuid string) (capabilities []string, privilegeLevel string, reported bool) {
	capabilityMu.RLock()
	defer capabilityMu.RUnlock()

	entry, ok := capabilityStore[uuid]
	if !ok || !entry.reported {
		return nil, "", false
	}
	return append([]string(nil), entry.capabilities...), entry.privilegeLevel, true
}

// ClientPrivilegeLevel returns the coarse privilege level an agent reported
// ("elevated", "standard" or "unknown"), or "" when nothing was reported.
func ClientPrivilegeLevel(uuid string) string {
	_, privilegeLevel, _ := ClientCapabilities(uuid)
	return privilegeLevel
}

// ForgetClientCapabilities drops runtime-only capability metadata for a client.
func ForgetClientCapabilities(uuid string) {
	capabilityMu.Lock()
	defer capabilityMu.Unlock()
	delete(capabilityStore, uuid)
}

// HasCapability reports whether a non-remote capability was advertised.
// Remote control is permanently unavailable even for legacy agents that never
// reported a capability list.
func HasCapability(uuid, capability string) bool {
	if capability == "" {
		return true
	}
	if isRemoteControlCapability(capability) {
		return false
	}
	capabilities, _, reported := ClientCapabilities(uuid)
	if !reported {
		return true
	}
	for _, entry := range capabilities {
		if entry == capability {
			return true
		}
	}
	return false
}

func isRemoteControlCapability(capability string) bool {
	switch capability {
	case CapabilityExec, CapabilityTerminal, CapabilityFile:
		return true
	default:
		return false
	}
}

// requiredCapability maps a remote control method to the capability an agent
// must have reported. Monitoring methods (ping, message, event) have no
// requirement and stay available to every agent.
func requiredCapability(method string) string {
	switch method {
	case v2.MethodAgentExec:
		return CapabilityExec
	case v2.MethodAgentTerminal:
		return CapabilityTerminal
	case v2.MethodAgentFile:
		return CapabilityFile
	default:
		return ""
	}
}

// CheckMethodCapability is the final dispatch backstop. Remote-control methods
// are rejected for every agent, including old agents that claim support or did
// not report capabilities at all.
func CheckMethodCapability(uuid, method string) error {
	required := requiredCapability(method)
	if required == "" {
		return nil
	}
	return fmt.Errorf("%w: method %q is removed from Komari Stable for agent %s", ErrCapabilityUnavailable, method, uuid)
}
