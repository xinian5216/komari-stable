package client

import (
	"testing"

	v2 "github.com/komari-monitor/komari/protocol/v2"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// TestIngestReportedCapabilitiesFromAgentReport covers the transport both modes
// use: WebSocket and POST agents send their capabilities inside agent.report.
func TestIngestReportedCapabilitiesFromAgentReport(t *testing.T) {
	const uuid = "ingest-report"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	report := v2.Report{
		Capabilities:   []string{"ping", "message", "event"},
		PrivilegeLevel: "elevated",
	}
	ingestReportedCapabilities(uuid, report.Capabilities, report.PrivilegeLevel)

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

// TestReportsWithoutCapabilitiesAreIgnored covers the legacy path: agents that
// send no capability fields leave the store untouched.
func TestReportsWithoutCapabilitiesAreIgnored(t *testing.T) {
	const uuid = "ingest-legacy"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	ingestReportedCapabilities(uuid, nil, "")
	if _, _, reported := agent_runtime.ClientCapabilities(uuid); reported {
		t.Fatal("a client that reported nothing is marked as reported")
	}
	if !agent_runtime.HasCapability(uuid, agent_runtime.CapabilityExec) {
		t.Fatal("legacy clients must keep the historical behaviour")
	}
}

// TestIngestReportedCapabilitiesFromBasicInfo keeps the tolerant path that reads
// the fields from a basic info payload if they ever appear there.
func TestIngestReportedCapabilitiesFromBasicInfo(t *testing.T) {
	const uuid = "ingest-basic-info"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	ingestReportedCapabilitiesFromBasicInfo(uuid, map[string]interface{}{
		"capabilities":    []interface{}{"ping", "message", "event"},
		"privilege_level": "standard",
	})

	capabilities, _, reported := agent_runtime.ClientCapabilities(uuid)
	if !reported {
		t.Fatal("capabilities were not recorded")
	}
	if len(capabilities) != 3 {
		t.Fatalf("capabilities = %v", capabilities)
	}

	// Nothing to read: the store stays untouched.
	ingestReportedCapabilitiesFromBasicInfo("ingest-empty", map[string]interface{}{"version": "1.5.10"})
	if _, _, reported := agent_runtime.ClientCapabilities("ingest-empty"); reported {
		t.Fatal("an empty payload was recorded")
	}
}

// TestPullParamsCapabilitiesAreIngested mirrors the POST fallback path.
func TestPullParamsCapabilitiesAreIngested(t *testing.T) {
	const uuid = "ingest-pull"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })

	params := v2.PullParams{Capabilities: []string{"ping", "message", "event", "exec", "terminal", "file"}}
	ingestReportedCapabilities(uuid, params.Capabilities, "")

	if !agent_runtime.HasCapability(uuid, agent_runtime.CapabilityFile) {
		t.Fatal("the reported file capability was not recorded")
	}
}
