// Package bundledtheme wires the theme package embedded at build time
// (web/public/bundledTheme, prepared from bundled-themes.lock.json) into the
// fresh-install path.
//
// Scope rules:
//   - only the installation wizard calls Seed();
//   - no server startup stage, upgrade or migration path calls it, so an existing
//     instance never gains data/theme/<short> and never changes its theme;
//   - a user who deletes the seeded theme keeps it deleted, because nothing
//     re-creates it.
package bundledtheme

import (
	"fmt"
	"strings"

	"github.com/komari-monitor/komari/internal/themebundle"
	"github.com/komari-monitor/komari/web/public"
)

// ThemesRoot is the installed-themes directory, relative to the working directory.
const ThemesRoot = "./data/theme"

// Seed installs the bundled preferred theme, if this build carries one, and
// reports what is now on disk. A build without a prepared bundle returns an error
// and changes nothing.
func Seed() (themebundle.Result, error) {
	return SeedInto(ThemesRoot)
}

// SeedInto is Seed with an explicit themes root, used by tests.
func SeedInto(themesRoot string) (themebundle.Result, error) {
	zipBytes, provenance, err := public.BundledTheme()
	if err != nil {
		return themebundle.Result{}, err
	}

	result, err := themebundle.Seed(zipBytes, provenance.SHA256, themesRoot)
	if err != nil {
		return themebundle.Result{}, err
	}

	expectedShort := strings.TrimSpace(provenance.ThemeShort)
	if expectedShort != "" && result.Short != expectedShort {
		return themebundle.Result{}, fmt.Errorf("bundled theme declares short %q, provenance says %q", result.Short, expectedShort)
	}
	return result, nil
}
