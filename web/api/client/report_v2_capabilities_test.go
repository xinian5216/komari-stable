package client

import (
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// TestIngestReportedCapabilitiesFromBasicInfo covers the WebSocket mode, where
// the agent reports its capabilities inside the basic info payload.
func TestIngestReportedCapabilitiesFromBasicInfo(t *testing.T) {
	const uuid = "ingest-basic-info"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	ingestReportedCapabilities(uuid, map[string]interface{}{
		"capabilities":    []interface{}{"ping", "message", "event"},
		"privilege_level": "elevated",
	})

	capabilities, privilegeLevel, reported := agent_runtime.ClientCapabilities(uuid)
	if !reported {
		t.Fatal("capabilities were not recorded")
	}
	if len(capabilities) != 3 || capabilities[0] != "ping" {
		t.Fatalf("capabilities = %v", capabilities)
	}
	if privilegeLevel != "elevated" {
		t.Fatalf("privilege level = %q", privilegeLevel)
	}
	if agent_runtime.HasCapability(uuid, agent_runtime.CapabilityTerminal) {
		t.Fatal("a monitoring-only agent must not be allowed to open a terminal")
	}
}

// TestIngestReportedCapabilitiesIgnoresAgentsThatDoNotReport covers the legacy
// path: nothing is stored, so the server keeps its historical behaviour.
func TestIngestReportedCapabilitiesIgnoresAgentsThatDoNotReport(t *testing.T) {
	const uuid = "ingest-legacy"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	ingestReportedCapabilities(uuid, map[string]interface{}{"version": "1.5.10"})
	if _, _, reported := agent_runtime.ClientCapabilities(uuid); reported {
		t.Fatal("a client that reported nothing is marked as reported")
	}
	if !agent_runtime.HasCapability(uuid, agent_runtime.CapabilityExec) {
		t.Fatal("legacy clients must keep the historical behaviour")
	}
}

// TestPullParamsCapabilitiesAreIngested mirrors the POST fallback path.
func TestPullParamsCapabilitiesAreIngested(t *testing.T) {
	const uuid = "ingest-pull"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	params := v2.PullParams{Capabilities: []string{"ping", "message", "event", "exec", "terminal", "file"}}
	agent_runtime.SetClientCapabilities(uuid, params.Capabilities, "")

	if !agent_runtime.HasCapability(uuid, agent_runtime.CapabilityFile) {
		t.Fatal("the reported file capability was not recorded")
	}
}
