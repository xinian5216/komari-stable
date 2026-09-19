package jsonrpc

import (
	"testing"

	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestRemovedRemoteControlRPCMethodsAreNotRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, method := range rpc.ListMethods() {
		registered[method] = true
	}
	for _, method := range []string{
		"admin:exec",
		"admin:getTasks",
		"admin:getTaskById",
		"admin:getTasksByClientId",
		"admin:getSpecificTaskResult",
		"admin:getTaskResultsByTaskId",
		"admin:fileList",
		"admin:fileListRoots",
		"admin:fileStat",
		"admin:fileMkdir",
		"admin:fileDelete",
		"admin:fileMove",
		"admin:fileCopy",
		"admin:fileChmod",
		"admin:fileChown",
		"admin:fileSearch",
		"admin:getXtermjsSettings",
		"admin:setXtermjsSettings",
	} {
		if registered[method] {
			t.Errorf("removed method is still registered: %s", method)
		}
		response := rpc.Call(1, method, nil)
		if response.Error == nil || response.Error.Code != rpc.MethodNotFound {
			t.Errorf("%s returned %#v, want MethodNotFound", method, response)
		}
	}
}
