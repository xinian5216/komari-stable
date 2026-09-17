package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The three-asset shape of xinian5216/komari-next-stable@v1.4.19-stable.1: the
// theme package plus two checksum assets. Asset order in the API response must not
// matter, so every ordering is covered below.
func stableReleaseAssets(zipPosition string) []githubReleaseAsset {
	checksums := githubReleaseAsset{Name: "SHA256SUMS", BrowserDownloadURL: "https://example.invalid/SHA256SUMS"}
	single := githubReleaseAsset{Name: "dist-release.zip.sha256", BrowserDownloadURL: "https://example.invalid/dist-release.zip.sha256"}
	theme := githubReleaseAsset{Name: "dist-release.zip", BrowserDownloadURL: "https://example.invalid/dist-release.zip"}

	switch zipPosition {
	case "first":
		return []githubReleaseAsset{theme, single, checksums}
	case "last":
		return []githubReleaseAsset{checksums, single, theme}
	default:
		return []githubReleaseAsset{single, theme, checksums}
	}
}

func TestSelectThemeReleaseAssetPrefersDistReleaseZip(t *testing.T) {
	for _, position := range []string{"first", "middle", "last"} {
		t.Run("zip "+position, func(t *testing.T) {
			got, err := selectThemeReleaseAsset(stableReleaseAssets(position))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != "https://example.invalid/dist-release.zip" {
				t.Fatalf("selected %q, want the theme package", got)
			}
		})
	}

	// Same release with a differently ordered asset list must give the same answer.
	shuffled := []githubReleaseAsset{
		{Name: "SHA256SUMS", BrowserDownloadURL: "https://example.invalid/SHA256SUMS"},
		{Name: "dist-release.zip", BrowserDownloadURL: "https://example.invalid/dist-release.zip"},
		{Name: "dist-release.zip.sha256", BrowserDownloadURL: "https://example.invalid/dist-release.zip.sha256"},
	}
	first, _ := selectThemeReleaseAsset(stableReleaseAssets("first"))
	second, _ := selectThemeReleaseAsset(shuffled)
	if first != second {
		t.Fatalf("selection depends on asset order: %q vs %q", first, second)
	}
}

func TestSelectThemeReleaseAssetRejectsMultiAssetWithoutTheme(t *testing.T) {
	// Two plausible packages (plus a checksum) and no dist-release.zip: guessing
	// would be unsafe, so the selection must fail loudly.
	assets := []githubReleaseAsset{
		{Name: "komari-theme.zip", BrowserDownloadURL: "https://example.invalid/komari-theme.zip"},
		{Name: "legacy-theme.zip", BrowserDownloadURL: "https://example.invalid/legacy-theme.zip"},
		{Name: "SHA256SUMS", BrowserDownloadURL: "https://example.invalid/SHA256SUMS"},
	}
	if _, err := selectThemeReleaseAsset(assets); err == nil {
		t.Fatal("a multi-asset release without dist-release.zip must fail loudly")
	} else if !strings.Contains(err.Error(), "无法确定") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A release that only carries checksum/metadata assets is not a theme release at
// all - it must never be downloaded as one.
func TestSelectThemeReleaseAssetTreatsChecksumPairAsNoTheme(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "SHA256SUMS", BrowserDownloadURL: "https://example.invalid/SHA256SUMS"},
		{Name: "dist-release.zip.sha256", BrowserDownloadURL: "https://example.invalid/dist-release.zip.sha256"},
	}
	if _, err := selectThemeReleaseAsset(assets); err == nil {
		t.Fatal("checksum-only releases must not be treated as themes")
	} else if !strings.Contains(err.Error(), preferredThemeAssetName) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectThemeReleaseAssetRejectsChecksumOnlyRelease(t *testing.T) {
	assets := []githubReleaseAsset{{Name: "SHA256SUMS", BrowserDownloadURL: "https://example.invalid/SHA256SUMS"}}
	if _, err := selectThemeReleaseAsset(assets); err == nil {
		t.Fatal("a checksum-only release must never be treated as a theme package")
	} else if !strings.Contains(err.Error(), preferredThemeAssetName) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectThemeReleaseAssetLegacySingleAsset(t *testing.T) {
	assets := []githubReleaseAsset{{Name: "komari-theme-1.0.0.zip", BrowserDownloadURL: "https://example.invalid/legacy.zip"}}
	got, err := selectThemeReleaseAsset(assets)
	if err != nil {
		t.Fatalf("legacy single-asset releases must keep working: %v", err)
	}
	if got != "https://example.invalid/legacy.zip" {
		t.Fatalf("selected %q", got)
	}
}

func TestSelectThemeReleaseAssetEmpty(t *testing.T) {
	if _, err := selectThemeReleaseAsset(nil); err == nil {
		t.Fatal("an empty release must fail")
	}
	if _, err := selectThemeReleaseAsset([]githubReleaseAsset{{Name: "dist-release.zip"}}); err == nil {
		t.Fatal("an asset without a download URL must not be selected")
	}
}

func withStubbedGitHubAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	previous := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	t.Cleanup(func() {
		githubAPIBaseURL = previous
		server.Close()
	})
}

func TestGetGitHubReleaseDownloadURLUsesTheThemeAsset(t *testing.T) {
	withStubbedGitHubAPI(t, func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasSuffix(request.URL.Path, "/releases/latest") {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"assets": []map[string]string{
				{"name": "SHA256SUMS", "browser_download_url": "https://example.invalid/SHA256SUMS"},
				{"name": "dist-release.zip.sha256", "browser_download_url": "https://example.invalid/dist-release.zip.sha256"},
				{"name": "dist-release.zip", "browser_download_url": "https://example.invalid/dist-release.zip"},
			},
		})
	})

	got, err := getGitHubReleaseDownloadURL("xinian5216", "komari-next-stable")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.invalid/dist-release.zip" {
		t.Fatalf("selected %q, want the theme package", got)
	}
}

func TestGetGitHubReleaseDownloadURLRejectsChecksumOnlyRelease(t *testing.T) {
	withStubbedGitHubAPI(t, func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"assets": []map[string]string{
				{"name": "SHA256SUMS", "browser_download_url": "https://example.invalid/SHA256SUMS"},
			},
		})
	})

	if _, err := getGitHubReleaseDownloadURL("xinian5216", "komari-next-stable"); err == nil {
		t.Fatal("expected an error instead of downloading a checksum file")
	}
}

func TestGetGitHubReleaseDownloadURLSurfacesAPIErrors(t *testing.T) {
	withStubbedGitHubAPI(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	})

	if _, err := getGitHubReleaseDownloadURL("xinian5216", "komari-next-stable"); err == nil {
		t.Fatal("expected an HTTP status error")
	}
	if _, err := getGitHubReleaseDownloadURL("", ""); err == nil {
		t.Fatal("expected a validation error for empty owner/repo")
	}
}
