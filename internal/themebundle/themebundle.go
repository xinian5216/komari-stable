// Package themebundle validates and installs a Komari theme package (zip) that is
// embedded in the binary at build time (see web/public/bundled.go and
// scripts/prepare-assets.py).
//
// The validation rules intentionally mirror the admin theme uploader
// (web/api/admin/theme.go) but are implemented here on purpose: the bundled
// package is a build-time artifact and the admin upload/market/update paths must
// stay untouched. Nothing in this package talks to the network or touches an
// existing theme directory.
package themebundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// Mirrors the limits enforced by web/api/admin/theme.go.
	MaxFiles         = 10000
	MaxFileSize      = 128 << 20
	MaxExtractedSize = 512 << 20
	MaxManifestSize  = 1 << 20

	// ManifestName is the theme manifest that must sit at the archive root.
	ManifestName = "komari-theme.json"

	// reservedShort is refused because the embedded default theme owns it.
	reservedShort = "default"

	tempPrefix = ".tmp-themebundle-"
)

var shortPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Manifest is the subset of komari-theme.json this package validates.
type Manifest struct {
	Name        json.RawMessage `json:"name"`
	Short       string          `json:"short"`
	Version     string          `json:"version"`
	Author      string          `json:"author"`
	URL         string          `json:"url"`
	Preview     string          `json:"preview"`
	Description json.RawMessage `json:"description"`
}

// Result describes a seed attempt.
type Result struct {
	Short   string
	Version string
	Path    string
	// Created is true when this call extracted the package and published
	// Path itself. Only such a directory may be rolled back by the caller.
	Created bool
	// Skipped is true when the target theme directory already existed before
	// this call, in which case nothing was written.
	Skipped bool
}

// Rollback removes the theme directory this call created, and refuses to touch
// anything else - the caller never has to guess paths:
//   - it is a no-op unless Created is set (a pre-existing theme is never removed);
//   - Path must still have the <themesRoot>/<short> shape;
//   - the manifest on disk must still describe the same short, so a directory
//     that was replaced in the meantime is left alone.
func (r Result) Rollback() error {
	if !r.Created || r.Path == "" || r.Short == "" {
		return nil
	}
	if filepath.Base(filepath.Clean(r.Path)) != r.Short {
		return fmt.Errorf("refusing to remove %q: not a <themesRoot>/%s directory", r.Path, r.Short)
	}
	onDisk, err := LoadManifest(filepath.Join(r.Path, ManifestName))
	if err != nil {
		return fmt.Errorf("refusing to remove %q: %w", r.Path, err)
	}
	if onDisk.Short != r.Short {
		return fmt.Errorf("refusing to remove %q: manifest declares short %q, expected %q", r.Path, onDisk.Short, r.Short)
	}
	if err := os.RemoveAll(r.Path); err != nil {
		return fmt.Errorf("remove %q: %w", r.Path, err)
	}
	return nil
}

// HashMatches reports whether zipBytes hashes to the expected hex digest.
func HashMatches(zipBytes []byte, expectedSHA256 string) bool {
	expected := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expected == "" {
		return false
	}
	sum := sha256.Sum256(zipBytes)
	return hex.EncodeToString(sum[:]) == expected
}

func validateName(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("manifest is missing the name field")
	}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		if strings.TrimSpace(plain) == "" {
			return errors.New("manifest name is empty")
		}
		return nil
	}
	var localized map[string]string
	if err := json.Unmarshal(raw, &localized); err == nil {
		for _, value := range localized {
			if strings.TrimSpace(value) != "" {
				return nil
			}
		}
	}
	return errors.New("manifest name must be a string or a non-empty language map")
}

func validateManifest(manifest *Manifest) error {
	if err := validateName(manifest.Name); err != nil {
		return err
	}
	if manifest.Short == "" {
		return errors.New("manifest is missing the short field")
	}
	if !shortPattern.MatchString(manifest.Short) {
		return fmt.Errorf("manifest short %q may only contain letters, digits, underscore and hyphen", manifest.Short)
	}
	if manifest.Short == reservedShort {
		return fmt.Errorf("manifest short %q is reserved", reservedShort)
	}
	for field, value := range map[string]string{
		"version": manifest.Version,
		"author":  manifest.Author,
		"url":     manifest.URL,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("manifest is missing the %s field", field)
		}
	}
	return nil
}

// Inspect validates the hash and the archive layout and returns the manifest.
// It never writes to disk.
func Inspect(zipBytes []byte, expectedSHA256 string) (*Manifest, error) {
	if !HashMatches(zipBytes, expectedSHA256) {
		sum := sha256.Sum256(zipBytes)
		return nil, fmt.Errorf("bundle sha256 mismatch: got %s, want %s", hex.EncodeToString(sum[:]), strings.ToLower(expectedSHA256))
	}

	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("bundle is not a readable zip: %w", err)
	}
	files := reader.File
	if len(files) > MaxFiles {
		return nil, fmt.Errorf("bundle has %d entries, more than the %d allowed", len(files), MaxFiles)
	}

	var total uint64
	seen := map[string]bool{}
	for _, file := range files {
		name := file.Name
		if name == "" {
			return nil, errors.New("bundle contains an entry with an empty name")
		}
		if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || strings.Contains(name, ":\\") {
			return nil, fmt.Errorf("bundle entry %q uses an absolute path", name)
		}
		cleaned := strings.TrimPrefix(filepath.ToSlash(name), "./")
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
			return nil, fmt.Errorf("bundle entry %q escapes the archive root", name)
		}
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0 {
			return nil, fmt.Errorf("bundle entry %q is not a regular file or directory", name)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 > MaxFileSize {
			return nil, fmt.Errorf("bundle entry %q exceeds the %d byte limit", name, MaxFileSize)
		}
		total += file.UncompressedSize64
		if total > MaxExtractedSize {
			return nil, fmt.Errorf("bundle exceeds the %d byte extracted limit", MaxExtractedSize)
		}
		seen[cleaned] = true
	}

	if !seen[ManifestName] {
		return nil, fmt.Errorf("bundle is missing %s at the archive root", ManifestName)
	}
	if !seen["dist/index.html"] {
		return nil, errors.New("bundle is missing dist/index.html")
	}

	manifest, err := readManifestFromArchive(files)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func readManifestFromArchive(files []*zip.File) (*Manifest, error) {
	for _, file := range files {
		if filepath.ToSlash(file.Name) != ManifestName {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", ManifestName, err)
		}
		defer handle.Close()
		raw, err := io.ReadAll(io.LimitReader(handle, MaxManifestSize+1))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", ManifestName, err)
		}
		if len(raw) > MaxManifestSize {
			return nil, fmt.Errorf("%s exceeds the %d byte limit", ManifestName, MaxManifestSize)
		}
		var manifest Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("%s is not valid JSON: %w", ManifestName, err)
		}
		return &manifest, nil
	}
	return nil, fmt.Errorf("bundle is missing %s at the archive root", ManifestName)
}

// Extract writes the archive contents below destDir. Callers are expected to have
// run Inspect first; the safety checks are repeated here so extraction is safe on
// its own too.
func Extract(zipBytes []byte, destDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("bundle is not a readable zip: %w", err)
	}
	cleanRoot := filepath.Clean(destDir)

	for _, file := range reader.File {
		name := strings.TrimPrefix(filepath.ToSlash(file.Name), "./")
		if name == "" {
			continue
		}
		if name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			return fmt.Errorf("bundle entry %q escapes the archive root", file.Name)
		}
		if mode := file.Mode(); mode&os.ModeSymlink != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0 {
			return fmt.Errorf("bundle entry %q is not a regular file or directory", file.Name)
		}

		target := filepath.Join(cleanRoot, filepath.FromSlash(name))
		if !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) {
			return fmt.Errorf("bundle entry %q escapes the target directory", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		source, err := file.Open()
		if err != nil {
			return err
		}
		destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			_ = source.Close()
			return err
		}
		_, copyErr := io.Copy(destination, io.LimitReader(source, MaxFileSize+1))
		closeErr := destination.Close()
		_ = source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// LoadManifest reads and validates komari-theme.json from disk.
func LoadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxManifestSize {
		return nil, fmt.Errorf("%s exceeds the %d byte limit", path, MaxManifestSize)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// Seed installs the bundled package into themesRoot/<short> atomically.
//
// Behaviour:
//   - the hash must match expectedSHA256 and the archive must pass Inspect;
//   - the package is extracted into a temporary directory inside themesRoot,
//     re-validated there, and only then renamed into place;
//   - an existing themesRoot/<short> is never overwritten (Result.Skipped);
//   - on any failure the temporary directory is removed, so a half-extracted
//     theme is never left behind;
//   - a directory created by this call is reported as Result.Created so the
//     caller can roll it back if a later step of the same installation fails.
func Seed(zipBytes []byte, expectedSHA256, themesRoot string) (Result, error) {
	manifest, err := Inspect(zipBytes, expectedSHA256)
	if err != nil {
		return Result{}, err
	}

	target := filepath.Join(themesRoot, manifest.Short)
	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return Result{}, fmt.Errorf("%s exists and is not a directory", target)
		}
		return Result{Short: manifest.Short, Version: manifest.Version, Path: target, Skipped: true}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Result{}, err
	}

	if err := os.MkdirAll(themesRoot, 0o755); err != nil {
		return Result{}, err
	}
	cleanupStaleTempDirs(themesRoot)

	tempDir, err := os.MkdirTemp(themesRoot, tempPrefix)
	if err != nil {
		return Result{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := Extract(zipBytes, tempDir); err != nil {
		return Result{}, err
	}

	onDisk, err := LoadManifest(filepath.Join(tempDir, ManifestName))
	if err != nil {
		return Result{}, fmt.Errorf("extracted bundle failed validation: %w", err)
	}
	if onDisk.Short != manifest.Short {
		return Result{}, fmt.Errorf("extracted bundle declares short %q, expected %q", onDisk.Short, manifest.Short)
	}

	if err := os.Rename(tempDir, target); err != nil {
		return Result{}, err
	}
	published = true
	return Result{Short: manifest.Short, Version: manifest.Version, Path: target, Created: true}, nil
}

// cleanupStaleTempDirs removes leftovers from an interrupted previous attempt.
func cleanupStaleTempDirs(themesRoot string) {
	entries, err := os.ReadDir(themesRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), tempPrefix) {
			_ = os.RemoveAll(filepath.Join(themesRoot, entry.Name()))
		}
	}
}
