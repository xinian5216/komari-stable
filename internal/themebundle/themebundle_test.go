package themebundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode == 0 {
			header.SetMode(0o644)
		} else {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q): %v", entry.name, err)
		}
		if _, err := file.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func manifestBody(t *testing.T, overrides map[string]any) string {
	t.Helper()
	manifest := map[string]any{
		"name":    "Komari Next",
		"short":   "next",
		"version": "1.4.19",
		"author":  "tonyliuzj",
		"url":     "https://github.com/xinian5216/komari-next-stable",
	}
	for key, value := range overrides {
		if value == nil {
			delete(manifest, key)
			continue
		}
		manifest[key] = value
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return string(raw)
}

func validEntries(t *testing.T, overrides map[string]any) []zipEntry {
	t.Helper()
	return []zipEntry{
		{name: "komari-theme.json", body: manifestBody(t, overrides)},
		{name: "preview.png", body: "preview"},
		{name: "dist/index.html", body: "<html>next</html>"},
		{name: "dist/assets/app.js", body: "console.log(1)"},
	}
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func TestSeedInstallsBundledThemeAtomically(t *testing.T) {
	payload := buildZip(t, validEntries(t, nil))
	root := t.TempDir()

	result, err := Seed(payload, sha256Hex(payload), root)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if result.Short != "next" || result.Version != "1.4.19" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Skipped {
		t.Fatal("fresh seed must not be reported as skipped")
	}
	for _, relative := range []string{"komari-theme.json", "dist/index.html", "dist/assets/app.js"} {
		if _, err := os.Stat(filepath.Join(result.Path, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing %s after seed: %v", relative, err)
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), tempPrefix) {
			t.Fatalf("temporary directory left behind: %s", entry.Name())
		}
	}
}

func TestSeedNeverOverwritesAnExistingTheme(t *testing.T) {
	payload := buildZip(t, validEntries(t, nil))
	root := t.TempDir()

	existing := filepath.Join(root, "next")
	if err := os.MkdirAll(filepath.Join(existing, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(existing, "komari-theme.json")
	if err := os.WriteFile(sentinel, []byte(`{"short":"next","user":"kept"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Seed(payload, sha256Hex(payload), root)
	if err != nil {
		t.Fatalf("Seed on existing theme: %v", err)
	}
	if !result.Skipped {
		t.Fatal("an existing theme directory must be reported as skipped")
	}
	raw, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "user") {
		t.Fatalf("existing theme content was modified: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(existing, "dist", "index.html")); err == nil {
		t.Fatal("seed must not write into an existing theme directory")
	}
}

func TestSeedRejectsHashMismatch(t *testing.T) {
	payload := buildZip(t, validEntries(t, nil))
	root := t.TempDir()

	if _, err := Seed(payload, strings.Repeat("0", 64), root); err == nil {
		t.Fatal("expected a hash mismatch error")
	} else if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("nothing may be written on hash mismatch (entries=%d err=%v)", len(entries), err)
	}
}

func TestSeedRejectsUnreadableZip(t *testing.T) {
	payload := []byte("this is not a zip archive")
	if _, err := Seed(payload, sha256Hex(payload), t.TempDir()); err == nil {
		t.Fatal("expected an unreadable zip error")
	}
}

func TestInspectRejectsManifestProblems(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]any
		wantText  string
	}{
		{"missing manifest file", nil, "missing komari-theme.json"},
		{"missing short", map[string]any{"short": nil}, "short"},
		{"reserved short", map[string]any{"short": "default"}, "reserved"},
		{"invalid short", map[string]any{"short": "next/../evil"}, "may only contain"},
		{"missing version", map[string]any{"version": nil}, "version"},
		{"missing author", map[string]any{"author": nil}, "author"},
		{"missing url", map[string]any{"url": nil}, "url"},
		{"empty name", map[string]any{"name": "   "}, "name"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			entries := validEntries(t, testCase.overrides)
			if testCase.name == "missing manifest file" {
				entries = entries[1:]
			}
			if testCase.name == "missing short" {
				// delete the field instead of setting null
				entries[0].body = manifestBody(t, map[string]any{"short": nil})
			}
			payload := buildZip(t, entries)
			_, err := Inspect(payload, sha256Hex(payload))
			if err == nil {
				t.Fatalf("expected an error for %s", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.wantText) {
				t.Fatalf("error %q does not mention %q", err, testCase.wantText)
			}
		})
	}
}

func TestInspectRejectsUnsafeEntries(t *testing.T) {
	cases := []struct {
		name     string
		entry    zipEntry
		wantText string
	}{
		{"traversal", zipEntry{name: "../evil.txt", body: "x"}, "escapes"},
		{"nested traversal", zipEntry{name: "dist/../../evil.txt", body: "x"}, "escapes"},
		{"absolute path", zipEntry{name: "/etc/passwd", body: "x"}, "absolute"},
		{"symlink", zipEntry{name: "dist/link", body: "/etc/passwd", mode: os.ModeSymlink | 0o777}, "not a regular file"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			entries := append(validEntries(t, nil), testCase.entry)
			payload := buildZip(t, entries)
			_, err := Inspect(payload, sha256Hex(payload))
			if err == nil {
				t.Fatalf("expected an error for %s", testCase.name)
			}
			if !strings.Contains(err.Error(), testCase.wantText) {
				t.Fatalf("error %q does not mention %q", err, testCase.wantText)
			}
		})
	}
}

func TestInspectRejectsTooManyEntries(t *testing.T) {
	entries := validEntries(t, nil)
	for index := 0; index < MaxFiles; index++ {
		entries = append(entries, zipEntry{name: fmt.Sprintf("dist/filler-%d.txt", index), body: "x"})
	}
	payload := buildZip(t, entries)
	_, err := Inspect(payload, sha256Hex(payload))
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("expected a file count error, got %v", err)
	}
}

func TestInspectRequiresDistIndex(t *testing.T) {
	entries := []zipEntry{
		{name: "komari-theme.json", body: manifestBody(t, nil)},
		{name: "dist/assets/app.js", body: "1"},
	}
	payload := buildZip(t, entries)
	_, err := Inspect(payload, sha256Hex(payload))
	if err == nil || !strings.Contains(err.Error(), "dist/index.html") {
		t.Fatalf("expected a missing dist/index.html error, got %v", err)
	}
}

func TestExtractRefusesTraversalEvenWithoutInspect(t *testing.T) {
	payload := buildZip(t, []zipEntry{{name: "../evil.txt", body: "x"}})
	if err := Extract(payload, t.TempDir()); err == nil {
		t.Fatal("Extract must refuse traversal entries on its own")
	}
}

// Seed must tell the caller whether it created the directory or left an existing one
// alone, so a failed later step can roll back exactly what it created.
func TestSeedReportsCreatedThenSkipped(t *testing.T) {
	zipBytes := buildZip(t, validEntries(t, nil))
	root := t.TempDir()

	first, err := Seed(zipBytes, sha256Hex(zipBytes), root)
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if !first.Created || first.Skipped {
		t.Fatalf("first seed must report Created, got Created=%v Skipped=%v", first.Created, first.Skipped)
	}
	if first.Short != "next" || first.Path != filepath.Join(root, "next") {
		t.Fatalf("unexpected result %+v", first)
	}

	second, err := Seed(zipBytes, sha256Hex(zipBytes), root)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if second.Created || !second.Skipped {
		t.Fatalf("second seed must report Skipped, got Created=%v Skipped=%v", second.Created, second.Skipped)
	}
}

// Rollback removes the directory a seed created and is a no-op for a theme that was
// already there before the seed ran.
func TestRollbackRemovesOnlyDirectoriesSeedCreated(t *testing.T) {
	zipBytes := buildZip(t, validEntries(t, nil))
	root := t.TempDir()

	created, err := Seed(zipBytes, sha256Hex(zipBytes), root)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := created.Rollback(); err != nil {
		t.Fatalf("rollback of a created theme: %v", err)
	}
	if _, err := os.Stat(created.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("rollback must remove %s (err=%v)", created.Path, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("themes root must be empty again, got %v (err=%v)", entries, err)
	}

	// A second installation that finds a pre-existing theme must leave it alone.
	sentinel := filepath.Join(root, "next", "dist", "index.html")
	if _, err := Seed(zipBytes, sha256Hex(zipBytes), root); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	before, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("read sentinel: %v", err)
	}
	skipped, err := Seed(zipBytes, sha256Hex(zipBytes), root)
	if err != nil || !skipped.Skipped {
		t.Fatalf("expected a skipped seed, got %+v (err=%v)", skipped, err)
	}
	if err := skipped.Rollback(); err != nil {
		t.Fatalf("rollback of a skipped seed must be a no-op, got %v", err)
	}
	after, err := os.ReadFile(sentinel)
	if err != nil || string(before) != string(after) {
		t.Fatalf("a skipped theme must survive a rollback unchanged (err=%v)", err)
	}
}

// Rollback never deletes on a guess: it checks the layout and the manifest on disk.
func TestRollbackRefusesForeignDirectories(t *testing.T) {
	root := t.TempDir()

	// (a) the directory no longer holds the theme it claims
	foreign := filepath.Join(root, "next")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(foreign, ManifestName), []byte(`{"name":"Other","short":"other","version":"1.0.0","author":"x","url":"https://example.invalid"}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	result := Result{Short: "next", Path: foreign, Created: true}
	if err := result.Rollback(); err == nil {
		t.Fatal("rollback must refuse a directory whose manifest describes another theme")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("refused rollback must leave the directory alone: %v", err)
	}

	// (b) the path does not have the expected <root>/<short> shape
	mismatched := Result{Short: "next", Path: filepath.Join(root, "next", "nested"), Created: true}
	if err := mismatched.Rollback(); err == nil {
		t.Fatal("rollback must refuse a path that is not <themesRoot>/<short>")
	}

	// (c) a result that did not create anything is never rolled back
	noop := Result{Short: "next", Path: foreign, Skipped: true}
	if err := noop.Rollback(); err != nil {
		t.Fatalf("rollback without Created must be a no-op, got %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("no-op rollback must not delete anything: %v", err)
	}
}

func TestHashMatches(t *testing.T) {
	payload := []byte("payload")
	if !HashMatches(payload, strings.ToUpper(sha256Hex(payload))) {
		t.Fatal("hash comparison must ignore case and surrounding space")
	}
	if HashMatches(payload, "  ") || HashMatches(payload, strings.Repeat("a", 64)) {
		t.Fatal("empty or wrong hashes must not match")
	}
}
