package agent

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

// Capability names as reported by agents. They are duplicated here on purpose:
// the server has to keep working with agents that report nothing at all.
const (
	CapabilityPing     = "ping"
	CapabilityMessage  = "message"
	CapabilityEvent    = "event"
	CapabilityExec     = "exec"
	CapabilityTerminal = "terminal"
	CapabilityFile     = "file"
)

// ErrCapabilityUnavailable is returned when an agent explicitly reported that
// it does not offer the capability a request needs (for example an agent that
// runs with remote control disabled).
var ErrCapabilityUnavailable = errors.New("capability unavailable")

type reportedCapabilities struct {
	capabilities   []string
	privilegeLevel string
	// reported is false for agents that never told the server what they can do
	// (older agents, or a server that was restarted and has not seen a report
	// yet). Such clients keep the historical behaviour.
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
		if capability = strings.TrimSpace(capability); capability != "" {
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

// ForgetClientCapabilities drops what was reported for a client. It is used
// when no report has been seen for a while so the client falls back to the
// legacy behaviour instead of being blocked by stale information.
func ForgetClientCapabilities(uuid string) {
	capabilityMu.Lock()
	defer capabilityMu.Unlock()
	delete(capabilityStore, uuid)
}

// HasCapability reports whether a method may be sent to the agent. Agents that
// reported nothing are always allowed: the server must not suddenly stop
// sending remote control commands to installations that predate capabilities.
func HasCapability(uuid, capability string) bool {
	if capability == "" {
		return true
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

// CheckMethodCapability returns ErrCapabilityUnavailable when the agent told the
// server that it cannot handle method. It is the single place the server
// decides whether a remote control command may be sent.
func CheckMethodCapability(uuid, method string) error {
	required := requiredCapability(method)
	if required == "" || HasCapability(uuid, required) {
		return nil
	}
	return fmt.Errorf("%w: agent %s reports no %q capability (remote control is disabled on the agent)", ErrCapabilityUnavailable, uuid, required)
}
