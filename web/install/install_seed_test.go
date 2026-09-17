package install

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/themebundle"
	"github.com/komari-monitor/komari/web/public"
	"gorm.io/gorm"
)

// stubSeed swaps the bundled-theme seeder for the duration of a test.
func stubSeed(t *testing.T, fn func() (themebundle.Result, error)) {
	t.Helper()
	previous := seedPreferredTheme
	seedPreferredTheme = fn
	t.Cleanup(func() { seedPreferredTheme = previous })
}

// stubSetMany swaps the settings writer, so a failed write can be simulated
// without breaking the test database.
func stubSetMany(t *testing.T, fn func(map[string]any) error) {
	t.Helper()
	previous := setManySettings
	setManySettings = fn
	t.Cleanup(func() { setManySettings = previous })
}

// requireBundledTheme skips the test when this build has no prepared bundle.
func requireBundledTheme(t *testing.T) {
	t.Helper()
	if _, _, err := public.BundledTheme(); err != nil {
		t.Skipf("bundled theme asset is not prepared in this build: %v", err)
	}
}

func validInstallRequest() completeRequest {
	return completeRequest{
		Username:    "owner",
		Password:    "Correct-horse-battery-staple1",
		Sitename:    "My Komari",
		Description: "Private monitoring",
		MetricDSN:   "file:" + filepath.ToSlash(filepath.Join(os.TempDir(), "komari-install-test-metrics.db")) + "?mode=rwc",
	}
}

func countUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	return count
}

func themeSetting(t *testing.T) any {
	t.Helper()
	value, err := appconfig.GetAs[string](appconfig.ThemeKey)
	if err != nil {
		return nil
	}
	return value
}

func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	digests := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digests[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return digests
}

// A fresh instance whose bundled theme seeds successfully becomes a Next instance.
func TestFreshInstallAdoptsBundledThemeWhenSeedSucceeds(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{Short: "next", Version: "1.4.19", Path: "data/theme/next", Created: true}, nil
	})

	settings := map[string]any{"sitename": "example"}
	if _, warning := applyBundledThemeForFreshInstall(settings); warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if got := settings[appconfig.ThemeKey]; got != "next" {
		t.Fatalf("theme setting = %v, want next", got)
	}
}

// A seed failure must never fail the installation: no theme key is written, so the
// built-in default applies.
func TestFreshInstallKeepsDefaultWhenSeedFails(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{}, errors.New("bundle sha256 mismatch")
	})

	settings := map[string]any{"sitename": "example"}
	if _, warning := applyBundledThemeForFreshInstall(settings); warning == "" {
		t.Fatal("a seed failure must be reported as a warning")
	}
	if _, exists := settings[appconfig.ThemeKey]; exists {
		t.Fatalf("theme must stay unset when seeding fails, got %v", settings[appconfig.ThemeKey])
	}
}

// A theme directory that already existed when the installation started is adopted
// only when it is a usable theme package, and its contents are never rewritten.
func TestFreshInstallAdoptsPreExistingUsableThemeWithoutTouchingIt(t *testing.T) {
	t.Chdir(t.TempDir())
	root := filepath.Join("data", "theme", "next")
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatalf("create pre-existing theme: %v", err)
	}
	manifest := `{"name":"Komari Next","short":"next","version":"0.0.1-user","author":"tonyliuzj","url":"https://example.invalid"}`
	if err := os.WriteFile(filepath.Join(root, "komari-theme.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write pre-existing manifest: %v", err)
	}

	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{Short: "next", Version: "1.4.19", Path: root, Skipped: true}, nil
	})

	settings := map[string]any{}
	if _, warning := applyBundledThemeForFreshInstall(settings); warning != "" {
		t.Fatalf("a usable pre-existing theme must be adopted, got warning %s", warning)
	}
	if got := settings[appconfig.ThemeKey]; got != "next" {
		t.Fatalf("theme setting = %v, want next", got)
	}
	raw, err := os.ReadFile(filepath.Join(root, "komari-theme.json"))
	if err != nil || string(raw) != manifest {
		t.Fatalf("pre-existing manifest was modified: %v %s", err, raw)
	}
}

// A pre-existing directory that is not a usable theme package keeps the instance on
// the default theme instead of being adopted.
func TestFreshInstallIgnoresUnusablePreExistingDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	root := filepath.Join("data", "theme", "next")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create pre-existing directory: %v", err)
	}

	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{Short: "next", Version: "1.4.19", Path: root, Skipped: true}, nil
	})

	settings := map[string]any{}
	if _, warning := applyBundledThemeForFreshInstall(settings); warning == "" {
		t.Fatal("an unusable pre-existing directory must be reported")
	}
	if _, exists := settings[appconfig.ThemeKey]; exists {
		t.Fatalf("theme must stay unset, got %v", settings[appconfig.ThemeKey])
	}
}

// A build without a prepared bundle simply never seeds.
func TestFreshInstallWithoutBundleReportsWarning(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{}, errors.New("bundled preferred theme is not present in this build")
	})

	settings := map[string]any{}
	if _, warning := applyBundledThemeForFreshInstall(settings); warning == "" {
		t.Fatal("expected a warning for a build without a bundled theme")
	}
	if len(settings) != 0 {
		t.Fatalf("settings must stay empty, got %v", settings)
	}
}

// Regression: a failed settings write must roll back both the account and the theme
// directory this installation seeded, so the next attempt starts clean and still
// ends up with theme=next.
func TestFreshInstallRetryAfterSettingsFailureSeedsAgain(t *testing.T) {
	requireBundledTheme(t)
	r, db, _ := setupInstallRouter(t)

	stubSetMany(t, func(map[string]any) error { return errors.New("simulated settings write failure") })
	first := performJSON(r, http.MethodPost, APIPath+"/complete", validInstallRequest())
	if first.Code == http.StatusOK {
		t.Fatalf("installation must fail when the settings write fails: %s", first.Body.String())
	}
	if users := countUsers(t, db); users != 0 {
		t.Fatalf("the account must be rolled back, %d user(s) left", users)
	}
	seeded := filepath.Join("data", "theme", "next")
	if _, err := os.Stat(seeded); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the theme seeded by this installation must be rolled back, %s still exists (err=%v)", seeded, err)
	}

	// Retry with the real settings writer: the seed must run again and win.
	stubSetMany(t, appconfig.SetMany)
	second := performJSON(r, http.MethodPost, APIPath+"/complete", validInstallRequest())
	if second.Code != http.StatusOK {
		t.Fatalf("retry status = %d: %s", second.Code, second.Body.String())
	}
	if users := countUsers(t, db); users != 1 {
		t.Fatalf("retry must create exactly one account, got %d", users)
	}
	manifest, err := themebundle.LoadManifest(filepath.Join(seeded, themebundle.ManifestName))
	if err != nil {
		t.Fatalf("retry did not seed the bundled theme: %v", err)
	}
	if manifest.Short != "next" {
		t.Fatalf("seeded theme short = %q, want next", manifest.Short)
	}
	if got := themeSetting(t); got != "next" {
		t.Fatalf("theme setting after retry = %v, want next", got)
	}
}

// A theme directory that existed before the installation started must survive a
// failed settings write completely unchanged, and a retry adopts it as it is.
func TestPreExistingThemeSurvivesSettingsFailure(t *testing.T) {
	requireBundledTheme(t)
	r, db, _ := setupInstallRouter(t)

	root := filepath.Join("data", "theme", "next")
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatalf("create pre-existing theme: %v", err)
	}
	manifest := `{"name":"Komari Next","short":"next","version":"0.0.1-user","author":"tonyliuzj","url":"https://example.invalid"}`
	if err := os.WriteFile(filepath.Join(root, "komari-theme.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write pre-existing manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist", "index.html"), []byte("<html>user theme</html>"), 0o644); err != nil {
		t.Fatalf("write pre-existing dist: %v", err)
	}
	before := treeDigest(t, root)

	stubSetMany(t, func(map[string]any) error { return errors.New("simulated settings write failure") })
	failed := performJSON(r, http.MethodPost, APIPath+"/complete", validInstallRequest())
	if failed.Code == http.StatusOK {
		t.Fatalf("installation must fail when the settings write fails: %s", failed.Body.String())
	}
	if users := countUsers(t, db); users != 0 {
		t.Fatalf("the account must be rolled back, %d user(s) left", users)
	}

	after := treeDigest(t, root)
	if len(before) != len(after) {
		t.Fatalf("pre-existing theme file count changed: %d -> %d", len(before), len(after))
	}
	for name, content := range before {
		if after[name] != content {
			t.Fatalf("pre-existing theme file %s was modified", name)
		}
	}

	// Retry: the pre-existing theme is adopted as it is, still byte-identical.
	stubSetMany(t, appconfig.SetMany)
	retry := performJSON(r, http.MethodPost, APIPath+"/complete", validInstallRequest())
	if retry.Code != http.StatusOK {
		t.Fatalf("retry status = %d: %s", retry.Code, retry.Body.String())
	}
	if got := themeSetting(t); got != "next" {
		t.Fatalf("theme setting after retry = %v, want next", got)
	}
	final := treeDigest(t, root)
	for name, content := range before {
		if final[name] != content {
			t.Fatalf("pre-existing theme file %s was modified by the retry", name)
		}
	}
}
