package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
)

type testRegistryRegistrar struct{}

func (testRegistryRegistrar) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/registry/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

func TestRouterAllowedOriginWildcard(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "https://foo.example.com")
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://foo.example.com" {
		t.Fatalf("allow origin = %q, want %q", got, "https://foo.example.com")
	}
}

func TestRouterAllowedOriginHostMatch(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"foo.example.com"})

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "https://foo.example.com:8443")
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://foo.example.com:8443" {
		t.Fatalf("allow origin = %q, want %q", got, "https://foo.example.com:8443")
	}
}

func TestRouterRejectsWildcardBypassOrigin(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "https://evilexample.com")
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("evilexample.com should be rejected, got Allow-Origin = %q", got)
	}
}

func TestRouterRejectsInvalidStreamID(t *testing.T) {
	router := &api.Router{}
	called := false

	handler := router.HandleWithID(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/stream/bad/status", nil)
	req.SetPathValue("id", "bad")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatalf("handler should not be called for invalid stream id")
	}
}

func TestStaticHTMLUsesNoStoreCacheControl(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control for / = %q, want %q", got, "no-store")
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/download.html", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control for html = %q, want %q", got, "no-store")
	}
}

func TestStaticJSDoesNotForceNoStore(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got == "no-store" {
		t.Fatalf("cache-control for js should not be no-store")
	}
}

func TestSecurityHeadersMiddlewareSetsCSP(t *testing.T) {
	h := api.SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf("content-security-policy header missing")
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf("csp missing script-src self: %q", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' https: http: ws: wss:") {
		t.Fatalf("csp missing expected connect-src policy: %q", csp)
	}
	if strings.Contains(csp, "connect-src *") {
		t.Fatalf("csp should not allow wildcard connect-src: %q", csp)
	}
}

func TestRateLimitSkipPathsAndStreamPathBehavior(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)
	router.SetWebSocketHandler(func(w http.ResponseWriter, r *http.Request, streamID string) {
		w.WriteHeader(http.StatusOK)
	})
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first version request status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/ping", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("ping endpoint should bypass rate limit")
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/download", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("download endpoint should bypass rate limit")
	}

	req = httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", strings.NewReader("x"))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("upload endpoint should bypass rate limit")
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("stream websocket endpoint should be rate limited, got %d", rec.Code)
	}
}

func TestResultsPageServesNoStoreWhenResultsHandlerEnabled(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	store, err := results.New(t.TempDir()+"/results.db", 10)
	if err != nil {
		t.Fatalf("results.New: %v", err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/results/abc12345", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("content-type = %q, want text/html", contentType)
	}
}

func TestRegistryRoutesRateLimited(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)
	h := router.SetupRoutes(testRegistryRegistrar{})

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/registry/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first registry request status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/registry/health", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second registry request status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestResultsPageRouteRateLimited(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)

	store, err := results.New(t.TempDir()+"/results.db", 10)
	if err != nil {
		t.Fatalf("results.New: %v", err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/results/abc12345", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first results page status = %d, want %d", rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/results/abc12345", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second results page status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRouterStaticFileServerAllowlist(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/embed.go", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("embed.go should be denied by allowlist, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/skill.html", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("skill.html should be served, got %d", rec.Code)
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
		{name: "health", method: http.MethodGet, path: "/health"},
		{name: "version", method: http.MethodGet, path: "/api/v1/version"},
		{name: "ping", method: http.MethodGet, path: "/api/v1/ping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://example.com"+tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.path, rec.Code, http.StatusOK)
			}
		})
	}
}
