package install

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/metricstore"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// cleanupEmptyRuntimeDirs removes runtime directories a test run created in the
// source tree, and only when they are empty. A directory with any content is left
// untouched, so this can never delete real data.
func cleanupEmptyRuntimeDirs(t *testing.T, root string) {
	t.Helper()
	if root == "" {
		return
	}
	for _, dir := range []string{filepath.Join(root, "data", "theme"), filepath.Join(root, "data")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return // does not exist (any more): nothing to clean up
		}
		if len(entries) != 0 {
			return // holds content, never touch it
		}
		if err := os.Remove(dir); err != nil {
			t.Logf("could not remove the empty directory %s: %v", dir, err)
		}
	}
}

func setupInstallRouter(t *testing.T) (*gin.Engine, *gorm.DB, *Controller) {
	t.Helper()
	// web/public extracts nothing but still mkdirs ./data/theme from its package
	// init, which runs before this test in the original working directory. Remember
	// that directory so the run can leave the source tree exactly as it found it.
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	// Everything the installer writes (data/theme/<short>, ...) is relative to the
	// working directory, so run each installer test inside its own temporary
	// directory instead of polluting the source tree.
	t.Chdir(t.TempDir())
	t.Cleanup(func() { cleanupEmptyRuntimeDirs(t, original) })
	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "install.db"))+"?mode=rwc"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open install database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &appconfig.ConfigItem{}); err != nil {
		t.Fatalf("migrate install database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get install sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	appconfig.SetDb(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	controller := NewController(db)
	controller.Activate()
	controller.Register(r)
	return r, db, controller
}

func performJSON(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

func TestInstallRejectsInvalidInputWithoutCreatingAccount(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "short", Sitename: "Komari",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid install status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("invalid install created users: count=%d err=%v", count, err)
	}
}

func TestInstallRejectsWeakPasswordWithoutCreatingAccount(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "lowercaseonly1", Sitename: "Komari", MetricDSN: "./data/metrics.db",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("weak password created users: count=%d err=%v", count, err)
	}
}

func TestInstallCompletesAndPersistsSettings(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	metricDSN := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "metrics.db")) + "?mode=rwc"
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username:    "owner",
		Password:    "Correct-horse-battery-staple1",
		Sitename:    "My Komari",
		Description: "Private monitoring",
		MetricDSN:   metricDSN,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("complete install status = %d: %s", response.Code, response.Body.String())
	}
	var user models.User
	if err := db.First(&user, "username = ?", "owner").Error; err != nil {
		t.Fatalf("find installed admin: %v", err)
	}
	want := map[string]any{
		appconfig.SitenameKey:         "My Komari",
		appconfig.DescriptionKey:      "Private monitoring",
		metricstore.MetricDBDriverKey: "sqlite",
		metricstore.MetricDBDSNKey:    metricDSN,
	}
	got, err := appconfig.GetAll()
	if err != nil {
		t.Fatalf("read all install settings: %v", err)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("setting %s = %#v, want %#v", key, got[key], value)
		}
	}

	repeat := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "other", Password: "Another-password1", Sitename: "Other", MetricDSN: "./data/metrics.db",
	})
	if repeat.Code != http.StatusConflict {
		t.Fatalf("repeat install status = %d, want %d", repeat.Code, http.StatusConflict)
	}
}

func TestInstallRejectsUnknownDSN(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "Strong-password1", Sitename: "Komari",
		MetricDSN: "not-a-recognized-dsn",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown DSN status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed DSN created users: count=%d err=%v", count, err)
	}
}
