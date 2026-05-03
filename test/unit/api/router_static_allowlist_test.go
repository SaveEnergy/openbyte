package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestRouterStaticServesSpeedtestHTTPModules(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	h := router.SetupRoutes()

	for _, name := range []string{
		"speedtest-http.js",
		"speedtest-http-download.js",
		"speedtest-http-shared.js",
		"speedtest-http-upload.js",
		"network-probes.js",
		"speedtest-orchestrator.js",
	} {
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
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/embed.go", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf(routerEmbedDeniedFmt, rec.Code)
	}
}

func TestRouterStaticFileServerAllowlistServesFontsFromWebRoot(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

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
	router.SetWebRoot(webRoot)

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/fonts/dm-sans-latin.woff2", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFontServedFmt, rec.Code)
	}
}

func TestCriticalRoutesRespondOK(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	h := router.SetupRoutes()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "health", method: http.MethodGet, path: healthRoutePath},
		{name: "version", method: http.MethodGet, path: versionAPIPath},
		{name: "ping", method: http.MethodGet, path: pingAPIPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, exampleBaseURL+tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s "+statusWantFmt, tt.method, tt.path, rec.Code, http.StatusOK)
			}
		})
	}
}
