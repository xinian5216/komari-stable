package bundledtheme

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/komari-monitor/komari/internal/themebundle"
	"github.com/komari-monitor/komari/web/public"
)

// The shipped bundle must be structurally valid, declare short=next and point its
// update source at this mirror. Skipped when the build has no prepared bundle.
func TestSeedIntoInstallsTheShippedBundle(t *testing.T) {
	if _, _, err := public.BundledTheme(); errors.Is(err, public.ErrBundledThemeMissing) {
		t.Skip("bundled theme asset was not prepared for this build")
	}

	root := t.TempDir()
	result, err := SeedInto(root)
	if err != nil {
		t.Fatalf("SeedInto: %v", err)
	}
	if result.Short != "next" {
		t.Fatalf("seeded short = %q, want next", result.Short)
	}

	manifest, err := themebundle.LoadManifest(filepath.Join(result.Path, "komari-theme.json"))
	if err != nil {
		t.Fatalf("seeded manifest is invalid: %v", err)
	}
	if !strings.Contains(manifest.URL, "xinian5216/komari-next-stable") {
		t.Fatalf("seeded theme update source = %q, want this mirror", manifest.URL)
	}
	if manifest.Author == "" {
		t.Fatal("seeded manifest lost its author")
	}

	// Seeding twice must never rewrite what is already there.
	again, err := SeedInto(root)
	if err != nil {
		t.Fatalf("second SeedInto: %v", err)
	}
	if !again.Skipped {
		t.Fatal("an existing theme directory must be reported as skipped")
	}
}

// A missing bundle must surface as a change-nothing error.
func TestSeedIntoFailsWithoutBundle(t *testing.T) {
	if _, _, err := public.BundledTheme(); !errors.Is(err, public.ErrBundledThemeMissing) {
		t.Skip("this build ships a bundled theme; the missing-bundle path is not reachable")
	}
	root := t.TempDir()
	if _, err := SeedInto(root); err == nil {
		t.Fatal("expected an error when no bundle is embedded")
	}
}
