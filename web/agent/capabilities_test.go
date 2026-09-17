package agent

import (
	"errors"
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func resetCapabilities(t *testing.T) {
	t.Helper()
	capabilityMu.Lock()
	previous := capabilityStore
	capabilityStore = make(map[string]reportedCapabilities)
	capabilityMu.Unlock()
	t.Cleanup(func() {
		capabilityMu.Lock()
		capabilityStore = previous
		capabilityMu.Unlock()
	})
}

func TestMonitoringOnlyAgentIsRefusedRemoteControlMethods(t *testing.T) {
	resetCapabilities(t)
	const uuid = "monitoring-only"
	SetClientCapabilities(uuid, []string{"ping", "message", "event"}, "standard")

	for _, method := range []string{v2.MethodAgentExec, v2.MethodAgentTerminal, v2.MethodAgentFile} {
		err := CheckMethodCapability(uuid, method)
		if err == nil {
			t.Fatalf("%s was allowed for a monitoring-only agent", method)
		}
		if !errors.Is(err, ErrCapabilityUnavailable) {
			t.Fatalf("%s returned %v, want ErrCapabilityUnavailable", method, err)
		}
	}

	for _, method := range []string{v2.MethodAgentPing, v2.MethodAgentMessage, v2.MethodAgentEvent, v2.MethodAgentReport} {
		if err := CheckMethodCapability(uuid, method); err != nil {
			t.Fatalf("%s must stay available: %v", method, err)
		}
	}
}

func TestOptedInAgentIsAllowedRemoteControlMethods(t *testing.T) {
	resetCapabilities(t)
	const uuid = "remote-control"
	SetClientCapabilities(uuid, []string{"ping", "message", "event", "exec", "terminal", "file"}, "elevated")

	for _, method := range []string{v2.MethodAgentExec, v2.MethodAgentTerminal, v2.MethodAgentFile} {
		if err := CheckMethodCapability(uuid, method); err != nil {
			t.Fatalf("%s must be allowed: %v", method, err)
		}
	}
}

func TestLegacyAgentsKeepTheHistoricalBehaviour(t *testing.T) {
	resetCapabilities(t)
	const uuid = "legacy-agent"

	// Nothing was reported (older agent, or a restarted server).
	for _, method := range []string{v2.MethodAgentExec, v2.MethodAgentTerminal, v2.MethodAgentFile} {
		if err := CheckMethodCapability(uuid, method); err != nil {
			t.Fatalf("%s must not be blocked for an agent that reported nothing: %v", method, err)
		}
	}

	_, _, reported := ClientCapabilities(uuid)
	if reported {
		t.Fatal("an agent that reported nothing is marked as reported")
	}
}

func TestPrivilegeLevelSurvivesACapabilityOnlyReport(t *testing.T) {
	resetCapabilities(t)
	const uuid = "privilege"
	SetClientCapabilities(uuid, []string{"ping", "message", "event"}, "elevated")
	// The POST fallback only carries capabilities.
	SetClientCapabilities(uuid, []string{"ping", "message", "event", "exec", "terminal", "file"}, "")

	_, privilegeLevel, reported := ClientCapabilities(uuid)
	if !reported {
		t.Fatal("capabilities were not recorded")
	}
	if privilegeLevel != "elevated" {
		t.Fatalf("privilege level = %q, want the previously reported value", privilegeLevel)
	}
	if !HasCapability(uuid, CapabilityExec) {
		t.Fatal("the updated capability list was not applied")
	}
}

func TestDispatchV2EventRefusesIncapableAgent(t *testing.T) {
	resetCapabilities(t)
	const uuid = "dispatch-refused"
	SetClientCapabilities(uuid, []string{"ping", "message", "event"}, "standard")
	MarkV2Client(uuid)
	t.Cleanup(func() { DeleteConnectedClients(uuid) })

	err := DispatchV2Event(uuid, v2.MethodAgentTerminal, v2.TerminalRequestParams{RequestID: "req-1"})
	if !errors.Is(err, ErrCapabilityUnavailable) {
		t.Fatalf("DispatchV2Event returned %v, want ErrCapabilityUnavailable", err)
	}
	if events := TakeV2Events(uuid, nil, 10); len(events) != 0 {
		t.Fatalf("a refused command was still queued: %v", events)
	}
}

func TestDispatchV2EventQueuesForLegacyAgent(t *testing.T) {
	resetCapabilities(t)
	const uuid = "dispatch-legacy"
	MarkV2Client(uuid)
	t.Cleanup(func() { DeleteConnectedClients(uuid) })

	if err := DispatchV2Event(uuid, v2.MethodAgentTerminal, v2.TerminalRequestParams{RequestID: "req-2"}); err != nil {
		t.Fatalf("legacy dispatch failed: %v", err)
	}
	events := TakeV2Events(uuid, nil, 10)
	if len(events) != 1 || events[0].Method != v2.MethodAgentTerminal {
		t.Fatalf("legacy event was not queued: %v", events)
	}
}

func TestDispatchV2EventReportsOfflineAgents(t *testing.T) {
	resetCapabilities(t)
	if err := DispatchV2Event("nobody-home", v2.MethodAgentPing, v2.PingParams{}); !errors.Is(err, ErrAgentOffline) {
		t.Fatalf("DispatchV2Event returned %v, want ErrAgentOffline", err)
	}
}
