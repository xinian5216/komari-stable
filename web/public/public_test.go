package public

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"hyphen language": {
			input: "zh-CN",
			want:  "zh-CN",
		},
		"underscore language": {
			input: "zh_CN",
			want:  "zh-CN",
		},
		"reject script injection": {
			input: `zh-CN" autofocus`,
		},
		"reject too short": {
			input: "z",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeHTMLLanguage(tt.input); got != tt.want {
				t.Fatalf("normalizeHTMLLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		html     string
		language string
		want     string
	}{
		"replace existing lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: "zh-CN",
			want:     `<html lang="zh-CN"><head></head></html>`,
		},
		"insert missing lang": {
			html:     `<html><head></head></html>`,
			language: "ja_JP",
			want:     `<html lang="ja-JP"><head></head></html>`,
		},
		"ignore invalid lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: `zh-CN" autofocus`,
			want:     `<html lang="en"><head></head></html>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaceHTMLLanguage(tt.html, tt.language); got != tt.want {
				t.Fatalf("replaceHTMLLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbeddedDistDoesNotEmbedRawFiles(t *testing.T) {
	if _, err := PublicFS.ReadFile("defaultTheme/dist/index.html"); err == nil {
		t.Fatal("PublicFS still embeds the raw frontend files")
	}
	if content, ok := defaultDistFiles[IndexFile]; !ok || len(content) == 0 {
		t.Fatalf("embedded dist does not contain a non-empty %q", IndexFile)
	}
}

func TestStaticRestrictedDoesNotServeCustomAssetOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	assetPath := filepath.Join("data", "theme", "custom", "dist", "assets")
	if err := os.MkdirAll(assetPath, 0o755); err != nil {
		t.Fatalf("create custom theme asset directory: %v", err)
	}
	const assetName = "about-D4JKo971.css"
	if err := os.WriteFile(filepath.Join(assetPath, assetName), []byte("custom override"), 0o644); err != nil {
		t.Fatalf("write custom theme asset: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open config db: %v", err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, "custom"); err != nil {
		t.Fatalf("set custom theme: %v", err)
	}

	router := gin.New()
	StaticRestricted(router.Group("/"), func(handlers ...gin.HandlerFunc) {
		router.NoRoute(handlers...)
	})
	for _, requestPath := range []string{"/assets/" + assetName} {
		request := httptest.NewRequest("GET", requestPath, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != 200 {
			t.Fatalf("restricted asset %s status = %d, want 200", requestPath, recorder.Code)
		}
		body, err := io.ReadAll(recorder.Result().Body)
		if err != nil {
			t.Fatalf("read restricted asset %s: %v", requestPath, err)
		}
		if string(body) == "custom override" {
			t.Fatalf("restricted listener served a custom theme asset override for %s", requestPath)
		}
	}

	indexRequest := httptest.NewRequest("GET", "/database-recovery", nil)
	indexRecorder := httptest.NewRecorder()
	router.ServeHTTP(indexRecorder, indexRequest)
	indexBody, err := io.ReadAll(indexRecorder.Result().Body)
	if err != nil {
		t.Fatalf("read restricted index: %v", err)
	}
	if strings.Contains(string(indexBody), `vite-plugin-pwa:register-sw`) {
		t.Fatal("restricted index still registers a service worker")
	}
}

func TestHasPathPrefix(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		{path: "/admin", prefix: "/admin", want: true},
		{path: "/admin/", prefix: "/admin", want: true},
		{path: "/admin/dashboard", prefix: "/admin", want: true},
		{path: "/administration", prefix: "/admin", want: false},
		{path: "/terminal", prefix: "/terminal", want: true},
		{path: "/", prefix: "/admin", want: false},
	}
	for _, tt := range tests {
		if got := hasPathPrefix(tt.path, tt.prefix); got != tt.want {
			t.Fatalf("hasPathPrefix(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestIsEmbeddedCoreAssetPath(t *testing.T) {
	core := []string{"/sw.js", "/registerSW.js", "/workbox-efbd304a.js"}
	for _, path := range core {
		if !isEmbeddedCoreAssetPath(path) {
			t.Fatalf("isEmbeddedCoreAssetPath(%q) = false, want true", path)
		}
	}
	public := []string{"/assets/entry-index-abc.js", "/assets/chunk-foo.js", "/manifest.json", "/index.html"}
	for _, path := range public {
		if isEmbeddedCoreAssetPath(path) {
			t.Fatalf("isEmbeddedCoreAssetPath(%q) = true, want false", path)
		}
	}
}

func TestIsCoreFrontendPath(t *testing.T) {
	core := []string{
		"/admin",
		"/admin/dashboard",
		"/admin/database-migration",
		"/terminal",
		"/terminal/session",
		"/manage",
		"/manage/nodes",
		"/install",
		"/database-recovery",
	}
	for _, path := range core {
		if !isCoreFrontendPath(path) {
			t.Fatalf("isCoreFrontendPath(%q) = false, want true", path)
		}
	}
	public := []string{"/", "/instance/abc", "/plugin/foo", "/index.html", "/assets/entry-index.js"}
	for _, path := range public {
		if isCoreFrontendPath(path) {
			t.Fatalf("isCoreFrontendPath(%q) = true, want false", path)
		}
	}
}

func TestEnsureCoreRouteServiceWorker(t *testing.T) {
	workbox := []byte(`importScripts("workbox-efbd304a.js");self.registerRoute(new self.NavigationRoute("index.html"));`)
	patched := ensureCoreRouteServiceWorker(workbox)
	if !bytes.Contains(patched, []byte(coreRouteServiceWorkerBypassMarker)) {
		t.Fatal("patched service worker is missing the core-route bypass marker")
	}
	if !bytes.HasPrefix(patched, coreRouteServiceWorkerBypass) {
		t.Fatal("bypass must be prepended; do not rewrite the Workbox body")
	}
	if !bytes.Contains(patched, workbox) {
		t.Fatal("patched service worker dropped the original Workbox script")
	}
	if !bytes.Equal(ensureCoreRouteServiceWorker(patched), patched) {
		t.Fatal("core-route bypass was applied twice")
	}

	closed := ensureCoreRouteServiceWorker(nil)
	if !bytes.Contains(closed, []byte(coreRouteServiceWorkerBypassMarker)) {
		t.Fatal("empty upstream SW must fail closed with the bypass marker")
	}
	if bytes.Contains(closed, workbox) {
		t.Fatal("fail-closed SW must not keep a missing upstream body")
	}
	if !bytes.Contains(closed, []byte("skipWaiting")) {
		t.Fatal("fail-closed SW must activate immediately")
	}
	if !bytes.Equal(ensureCoreRouteServiceWorker([]byte(" \n\t")), closed) {
		t.Fatal("whitespace-only upstream SW must use the same fail-closed script")
	}
}

func TestThemeNextKeepsPublicHomeAndIsolatesCoreFrontend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())

	nextDist := filepath.Join("data", "theme", "next", "dist")
	if err := os.MkdirAll(filepath.Join(nextDist, "assets"), 0o755); err != nil {
		t.Fatalf("create next theme dist: %v", err)
	}
	const nextHome = `<!DOCTYPE html><html><body><div id="__next">NEXT_PUBLIC_HOME</div><script src="/_next/static/chunks/main.js"></script></body></html>`
	if err := os.WriteFile(filepath.Join(nextDist, "index.html"), []byte(nextHome), 0o644); err != nil {
		t.Fatalf("write next index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nextDist, "sw.js"), []byte("/* THEME_SW_POISON */"), 0o644); err != nil {
		t.Fatalf("write next sw.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nextDist, "registerSW.js"), []byte("/* THEME_REGISTER_POISON */"), 0o644); err != nil {
		t.Fatalf("write next registerSW.js: %v", err)
	}

	const themeOwnedEntry = "assets/entry-custom-theme.js"
	if err := os.WriteFile(filepath.Join(nextDist, filepath.FromSlash(themeOwnedEntry)), []byte("/* THEME_OWNED_ENTRY */"), 0o644); err != nil {
		t.Fatalf("write theme-owned entry: %v", err)
	}
	var hashedEntry string
	for name := range defaultDistFiles {
		base := path.Base(name)
		if strings.HasPrefix(name, "assets/") && strings.HasPrefix(base, "entry-") && strings.HasSuffix(base, ".js") {
			hashedEntry = name
			break
		}
	}
	if hashedEntry == "" {
		t.Fatal("embedded default frontend has no assets/entry-*.js file")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open config db: %v", err)
	}
	config.SetDb(db)
	if err := config.Set(config.ThemeKey, "next"); err != nil {
		t.Fatalf("set theme=next: %v", err)
	}

	router := gin.New()
	Static(router.Group("/"), func(handlers ...gin.HandlerFunc) {
		router.NoRoute(handlers...)
	})

	get := func(path string) (int, string) {
		t.Helper()
		request := httptest.NewRequest("GET", path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		body, err := io.ReadAll(recorder.Result().Body)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return recorder.Code, string(body)
	}

	if code, body := get("/"); code != 200 || !strings.Contains(body, "NEXT_PUBLIC_HOME") {
		t.Fatalf("GET / status=%d body=%q, want Next homepage", code, trimForTest(body))
	}
	if _, body := get("/"); strings.Contains(body, "entry-index-") {
		t.Fatal("GET / served the embedded default frontend")
	}

	corePaths := []string{"/admin", "/admin/dashboard", "/terminal", "/manage"}
	for _, path := range corePaths {
		code, body := get(path)
		if code != 200 {
			t.Fatalf("GET %s status = %d, want 200", path, code)
		}
		if !strings.Contains(body, "entry-index-") {
			t.Fatalf("GET %s is not the embedded default frontend", path)
		}
		if strings.Contains(body, "NEXT_PUBLIC_HOME") || strings.Contains(body, "__next") || strings.Contains(body, "_next/") {
			t.Fatalf("GET %s leaked the Next theme", path)
		}
		if strings.Contains(body, "vite-plugin-pwa:register-sw") || strings.Contains(body, `src="/registerSW.js"`) {
			t.Fatalf("GET %s still registers a root-scoped service worker", path)
		}
	}

	if code, body := get("/sw.js"); code != 200 {
		t.Fatalf("GET /sw.js status = %d", code)
	} else {
		if strings.Contains(body, "THEME_SW_POISON") {
			t.Fatal("GET /sw.js was shadowed by the Next theme")
		}
		if !strings.Contains(body, coreRouteServiceWorkerBypassMarker) {
			t.Fatal("GET /sw.js is missing the core-route bypass")
		}
	}

	if _, body := get("/registerSW.js"); strings.Contains(body, "THEME_REGISTER_POISON") {
		t.Fatal("GET /registerSW.js was shadowed by the Next theme")
	}

	if code, body := get("/" + themeOwnedEntry); code != 200 || !strings.Contains(body, "THEME_OWNED_ENTRY") {
		t.Fatalf("GET /%s status=%d body=%q, want the public theme file", themeOwnedEntry, code, trimForTest(body))
	}
	if code, body := get("/"+hashedEntry); code != 200 {
		t.Fatalf("GET /%s status = %d", hashedEntry, code)
	} else if strings.Contains(body, "NEXT_PUBLIC_HOME") || strings.Contains(body, "THEME_OWNED_ENTRY") {
		t.Fatalf("GET /%s did not fall back to the embedded default bundle", hashedEntry)
	}
}

func trimForTest(body string) string {
	if len(body) > 240 {
		return body[:240]
	}
	return body
}
