package jsonrpc

import (
	"testing"

	"github.com/komari-monitor/komari/database/models"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

func TestAttachReportedCapabilities(t *testing.T) {
	const uuid = "attach-caps"
	t.Cleanup(func() { agent_runtime.ForgetClientCapabilities(uuid) })
	agent_runtime.SetClientCapabilities(uuid, []string{"ping", "message", "event"}, "elevated")

	client := models.Client{UUID: uuid}
	attachReportedCapabilities(&client)

	if !client.RemoteControlKnown {
		t.Fatal("reported capabilities were not attached")
	}
	if client.PrivilegeLevel != "elevated" {
		t.Fatalf("privilege level = %q", client.PrivilegeLevel)
	}
	if len(client.Capabilities) != 3 {
		t.Fatalf("capabilities = %v", client.Capabilities)
	}
}

func TestAttachReportedCapabilitiesLeavesLegacyClientsAlone(t *testing.T) {
	client := models.Client{UUID: "attach-legacy"}
	attachReportedCapabilities(&client)

	if client.RemoteControlKnown || client.Capabilities != nil || client.PrivilegeLevel != "" {
		t.Fatalf("legacy client was modified: %+v", client)
	}
}
