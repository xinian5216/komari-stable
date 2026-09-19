package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRemovedRemoteControlRoutesReturnGone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerRemovedRemoteControlRoutes(router)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/terminal"},
		{http.MethodGet, "/terminal/session"},
		{http.MethodGet, "/api/clients/terminal?id=old"},
		{http.MethodPost, "/api/clients/transfer/old"},
		{http.MethodGet, "/api/preview/client/node/file/download"},
		{http.MethodPost, "/api/admin/task/exec"},
		{http.MethodGet, "/api/admin/task/old/result"},
		{http.MethodPost, "/api/admin/settings/xtermjs"},
		{http.MethodGet, "/api/admin/client/node/terminal"},
		{http.MethodPost, "/api/admin/client/node/file/upload"},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusGone {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGone)
			}
			if !strings.Contains(recorder.Body.String(), "remote control was removed") {
				t.Fatalf("unexpected response: %s", recorder.Body.String())
			}
		})
	}
}
