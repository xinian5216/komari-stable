package install

import (
	"errors"
	"testing"

	appconfig "github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/themebundle"
)

// stubSeed swaps the bundled-theme seeder for the duration of a test.
func stubSeed(t *testing.T, fn func() (themebundle.Result, error)) {
	t.Helper()
	previous := seedPreferredTheme
	seedPreferredTheme = fn
	t.Cleanup(func() { seedPreferredTheme = previous })
}

// A fresh instance whose bundled theme seeds successfully becomes a Next instance.
func TestFreshInstallAdoptsBundledThemeWhenSeedSucceeds(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{Short: "next", Version: "1.4.19", Path: "data/theme/next"}, nil
	})

	settings := map[string]any{"sitename": "example"}
	if warning := applyBundledThemeForFreshInstall(settings); warning != "" {
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
	warning := applyBundledThemeForFreshInstall(settings)
	if warning == "" {
		t.Fatal("a seed failure must be reported as a warning")
	}
	if _, exists := settings[appconfig.ThemeKey]; exists {
		t.Fatalf("theme must stay unset when seeding fails, got %v", settings[appconfig.ThemeKey])
	}
}

// An already seeded theme (for example a re-run right after a successful seed) is
// left untouched and the theme setting is not rewritten.
func TestFreshInstallLeavesExistingThemeSettingUntouched(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{Short: "next", Version: "1.4.19", Path: "data/theme/next", Skipped: true}, nil
	})

	settings := map[string]any{"sitename": "example"}
	warning := applyBundledThemeForFreshInstall(settings)
	if warning == "" {
		t.Fatal("a skipped seed must be reported")
	}
	if _, exists := settings[appconfig.ThemeKey]; exists {
		t.Fatal("the theme setting must not be written for a skipped seed")
	}
}

// A build without a prepared bundle simply never seeds.
func TestFreshInstallWithoutBundleReportsWarning(t *testing.T) {
	stubSeed(t, func() (themebundle.Result, error) {
		return themebundle.Result{}, errors.New("bundled preferred theme is not present in this build")
	})

	settings := map[string]any{}
	if warning := applyBundledThemeForFreshInstall(settings); warning == "" {
		t.Fatal("expected a warning for a build without a bundled theme")
	}
	if len(settings) != 0 {
		t.Fatalf("settings must stay empty, got %v", settings)
	}
}
