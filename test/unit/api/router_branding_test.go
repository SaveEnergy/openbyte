package api_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

func TestBrandingCSSServesValidatedThemeOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BrandPrimaryColorDark = "#00d4aa"
	cfg.BrandPrimaryColorLight = "#00796b"
	cfg.BrandSecondaryColorDark = "#667eea"
	cfg.BrandSecondaryColorLight = "#667eea"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	handler := api.NewRouter(cfg, nil).SetupRoutes()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
	css := rec.Body.String()
	for _, want := range []string{
		`:root {`,
		`--brand-primary: #00d4aa;`,
		`--brand-secondary: #1fd9b4;`,
		`--on-brand: #000000;`,
		`--accent-primary: #00d4aa;`,
		`--accent-secondary: #1fd9b4;`,
		`--accent-glow: rgba(0, 212, 170, 0.30);`,
		`--ambient-primary: rgba(0, 212, 170, 0.08);`,
		`--ambient-secondary: rgba(102, 126, 234, 0.05);`,
		`--upload-color: #667eea;`,
		`:root:not([data-theme="dark"])`,
		`:root[data-theme="light"]`,
		`--accent-primary: #00796b;`,
		`--on-brand: #ffffff;`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("branding CSS missing %q:\n%s", want, css)
		}
	}
	if strings.Contains(css, "color-mix") {
		t.Fatal("branding CSS should contain only precomputed color literals")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "style-src 'self'") {
		t.Fatalf("CSP changed unexpectedly: %q", csp)
	}
}

func TestBrandingCSSHeadAndDefaultResponse(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, exampleBaseURL+"/branding.css", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: "+statusWantFmt, method, rec.Code, http.StatusOK)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("%s default branding CSS body = %q, want empty", method, rec.Body.String())
		}
		if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
			t.Fatalf("%s cache-control = %q", method, got)
		}
	}
}

func TestBrandLogoServesStartupSnapshotAndHead(t *testing.T) {
	var logo bytes.Buffer
	if err := png.Encode(&logo, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode logo: %v", err)
	}
	path := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(path, logo.Bytes(), 0o600); err != nil {
		t.Fatalf("write logo: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.BrandLogoPath = path
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	handler := api.NewRouter(cfg, nil).SetupRoutes()
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatalf("replace logo: %v", err)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding/logo", nil))
	if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), logo.Bytes()) {
		t.Fatalf("GET logo status/body = %d/%d bytes", get.Code, get.Body.Len())
	}
	if got := get.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("logo content-type = %q", got)
	}
	if got := get.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("logo cache-control = %q", got)
	}
	dark := httptest.NewRecorder()
	handler.ServeHTTP(dark, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding/logo?theme=dark", nil))
	if dark.Code != http.StatusOK || !bytes.Equal(dark.Body.Bytes(), logo.Bytes()) {
		t.Fatal("single logo must also serve the dark theme")
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, exampleBaseURL+"/branding/logo", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD logo status/body = %d/%d bytes", head.Code, head.Body.Len())
	}

	css := httptest.NewRecorder()
	handler.ServeHTTP(css, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding.css", nil))
	for _, want := range []string{
		".brand-wordmark { display: none; }",
		".brand-logo { display: block; }",
	} {
		if !strings.Contains(css.Body.String(), want) {
			t.Errorf("logo-only branding CSS missing %q: %s", want, css.Body.String())
		}
	}
}

func TestBrandLogoReturnsNotFoundWhenUnset(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, exampleBaseURL+"/branding/logo", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestBrandLogosServeThemeSnapshots(t *testing.T) {
	var light, dark bytes.Buffer
	if err := png.Encode(&light, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&dark, image.NewNRGBA(image.Rect(0, 0, 3, 3)), nil); err != nil {
		t.Fatal(err)
	}
	lightPath := filepath.Join(t.TempDir(), "light.png")
	darkPath := filepath.Join(t.TempDir(), "dark.jpg")
	for path, data := range map[string][]byte{lightPath: light.Bytes(), darkPath: dark.Bytes()} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("BRAND_LOGO_PATH_LIGHT", lightPath)
	t.Setenv("BRAND_LOGO_PATH_DARK", darkPath)
	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"light", "dark"} {
		copy := cfg.BrandLogoForTheme(theme)
		copy.Data[0] ^= 0xff
		if bytes.Equal(copy.Data, cfg.BrandLogoForTheme(theme).Data) {
			t.Fatal("logo snapshot is mutable")
		}
	}
	handler := api.NewRouter(cfg, nil).SetupRoutes()
	for _, path := range []string{lightPath, darkPath} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		query, contentType string
		data               []byte
	}{
		{"", "image/png", light.Bytes()},
		{"?theme=light", "image/png", light.Bytes()},
		{"?theme=dark", "image/jpeg", dark.Bytes()},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, exampleBaseURL+"/branding/logo"+tc.query, nil))
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != tc.contentType || rec.Header().Get(cacheControlKey) != noStoreHeader {
				t.Fatalf("%s %s: status=%d headers=%v", method, tc.query, rec.Code, rec.Header())
			}
			if method == http.MethodGet && !bytes.Equal(rec.Body.Bytes(), tc.data) || method == http.MethodHead && rec.Body.Len() != 0 {
				t.Fatalf("%s %s: unexpected logo body", method, tc.query)
			}
		}
	}
}
