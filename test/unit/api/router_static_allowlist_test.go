package api_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/web"
)

func TestRouterStaticServesEmbeddedAssets(t *testing.T) {
	router := api.NewRouter(config.DefaultConfig(), nil)
	h := router.SetupRoutes()

	var assets []string
	if err := fs.WalkDir(web.Assets, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			assets = append(assets, name)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk embedded frontend assets: %v", err)
	}

	for _, name := range assets {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/"+name, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status %d, want %d", name, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestRouterStaticFileServerAllowlist(t *testing.T) {
	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf(routerWriteIndexFmt, err)
	}
	if err := os.WriteFile(filepath.Join(webRoot, "embed.go"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("write disallowed source: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.WebRoot = webRoot
	router := api.NewRouter(cfg, nil)

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/embed.go", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf(routerEmbedDeniedFmt, rec.Code)
	}
}

func TestRouterStaticFileServerAllowlistServesFontsFromWebRoot(t *testing.T) {
	webRoot := t.TempDir()
	fontDir := filepath.Join(webRoot, "fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatalf(routerMkdirFontsFmt, err)
	}
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf(routerWriteIndexFmt, err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "dm-sans-latin.woff2"), []byte("font-bytes"), 0o644); err != nil {
		t.Fatalf(routerWriteFontFmt, err)
	}
	cfg := config.DefaultConfig()
	cfg.WebRoot = webRoot
	router := api.NewRouter(cfg, nil)

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/fonts/dm-sans-latin.woff2", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFontServedFmt, rec.Code)
	}
}

func TestHealthRouteResponse(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+healthRoutePath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s "+statusWantFmt, healthRoutePath, rec.Code, http.StatusOK)
	}
	var response struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("health status = %q, want ok", response.Status)
	}
}

func TestRemovedBrowserAPIPagesReturnNotFound(t *testing.T) {
	handler := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	for _, path := range []string{"/api", "/api.html", "/api.css", "/api.js"} {
		req := httptest.NewRequest(http.MethodGet, exampleBaseURL+path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: "+statusWantFmt, path, rec.Code, http.StatusNotFound)
		}
	}
}
