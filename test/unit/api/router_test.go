package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
)

const (
	statusWantFmt             = "status = %d, want %d"
	exampleBaseURL            = "http://example.com"
	allowedOriginKey          = "Access-Control-Allow-Origin"
	cacheControlKey           = "Cache-Control"
	noStoreHeader             = "no-store"
	fooOrigin                 = "https://foo.example.com"
	fooOriginWithPort         = "https://foo.example.com:8443"
	resultsDBPath             = "/results.db"
	resultsNewErrFmt          = "results.New: %v"
	resultsPagePath           = "/results/abc12345"
	apiUnknownPath            = "/api/v1/nonexistent"
	registryHealthAPI         = "/api/v1/registry/health"
	versionAPIPath            = "/api/v1/version"
	pingAPIPath               = "/api/v1/ping"
	downloadAPIPath           = "/api/v1/download"
	uploadAPIPath             = "/api/v1/upload"
	streamWSAPIPath           = "/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream"
	healthRoutePath           = "/health"
	evilOrigin                = "https://evilexample.com"
	routerOctetStreamType     = "application/octet-stream"
	routerContentTypeKey      = "Content-Type"
	routerAllowOriginFmt      = "allow origin = %q, want %q"
	routerContentTypeJSON     = "application/json"
	routerContentTypeHTML     = "text/html"
	routerCacheRootFmt        = "cache-control for / = %q, want %q"
	routerCacheHTMLFmt        = "cache-control for html = %q, want %q"
	routerFirstVersionReq     = "first version request "
	routerFirstRegistryReq    = "first registry request "
	routerSecondRegistryReq   = "second registry request "
	routerFirstResultsReq     = "first results page "
	routerSecondResultsReq    = "second results page "
	routerEvilBypassFmt       = "evilexample.com should be rejected, got Allow-Origin = %q"
	routerInvalidIDCalledErr  = "handler should not be called for invalid stream id"
	routerJSNoStoreErr        = "cache-control for js should not be no-store"
	routerCSPHeaderMissingErr = "content-security-policy header missing"
	routerCSPScriptSrcFmt     = "csp missing script-src self: %q"
	routerCSPConnectSrcFmt    = "csp missing expected connect-src policy: %q"
	routerCSPWildcardErrFmt   = "csp should not allow wildcard connect-src: %q"
	routerPingBypassErr       = "ping endpoint should bypass rate limit"
	routerDownloadBypassErr   = "download endpoint should bypass rate limit"
	routerUploadBypassErr     = "upload endpoint should bypass rate limit"
	routerStreamRateLimitFmt  = "stream websocket endpoint should be rate limited, got %d"
	routerCacheControlFmt     = "cache-control = %q, want %q"
	routerBodyNotFoundFmt     = "body = %q, want not found JSON"
	routerContentTypeWantFmt  = "content-type = %q, want %s"
	routerMkdirFontsFmt       = "mkdir fonts: %v"
	routerWriteIndexFmt       = "write index.html: %v"
	routerWriteFontFmt        = "write font: %v"
	routerFontServedFmt       = "font should be served, got %d"
	routerEmbedDeniedFmt      = "embed.go should be denied by allowlist, got %d"
	routerSkillServedFmt      = "skill.html should be served, got %d"
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

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", fooOrigin)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(allowedOriginKey); got != fooOrigin {
		t.Fatalf(routerAllowOriginFmt, got, fooOrigin)
	}
}

func TestRouterAllowedOriginHostMatch(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"foo.example.com"})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", fooOriginWithPort)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(allowedOriginKey); got != fooOriginWithPort {
		t.Fatalf(routerAllowOriginFmt, got, fooOriginWithPort)
	}
}

func TestRouterRejectsWildcardBypassOrigin(t *testing.T) {
	router := &api.Router{}
	router.SetAllowedOrigins([]string{"*.example.com"})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL, nil)
	req.Header.Set("Origin", evilOrigin)
	rec := httptest.NewRecorder()

	handler := router.CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(allowedOriginKey); got != "" {
		t.Fatalf(routerEvilBypassFmt, got)
	}
}

func TestRouterRejectsInvalidStreamID(t *testing.T) {
	router := &api.Router{}
	called := false

	handler := router.HandleWithID(func(w http.ResponseWriter, r *http.Request, streamID string) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/api/v1/stream/bad/status", nil)
	req.SetPathValue("id", "bad")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatalf(routerInvalidIDCalledErr)
	}
}

func TestStaticHTMLUsesNoStoreCacheControl(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf(routerCacheRootFmt, got, noStoreHeader)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+"/download.html", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf(routerCacheHTMLFmt, got, noStoreHeader)
	}
}

func TestStaticJSDoesNotForceNoStore(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/openbyte.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get(cacheControlKey) == noStoreHeader {
		t.Fatalf(routerJSNoStoreErr)
	}
}

func TestSecurityHeadersMiddlewareSetsCSP(t *testing.T) {
	h := api.SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatalf(routerCSPHeaderMissingErr)
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Fatalf(routerCSPScriptSrcFmt, csp)
	}
	if !strings.Contains(csp, "connect-src 'self' https: http: ws: wss:") {
		t.Fatalf(routerCSPConnectSrcFmt, csp)
	}
	if strings.Contains(csp, "connect-src *") {
		t.Fatalf(routerCSPWildcardErrFmt, csp)
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

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+versionAPIPath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFirstVersionReq+statusWantFmt, rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+pingAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf(routerPingBypassErr)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+downloadAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf(routerDownloadBypassErr)
	}

	req = httptest.NewRequest(http.MethodPost, exampleBaseURL+uploadAPIPath, strings.NewReader("x"))
	req.Header.Set(routerContentTypeKey, routerOctetStreamType)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf(routerUploadBypassErr)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+streamWSAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(routerStreamRateLimitFmt, rec.Code)
	}
}

func TestResultsPageServesNoStoreWhenResultsHandlerEnabled(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	store, err := results.New(t.TempDir()+resultsDBPath, 10)
	if err != nil {
		t.Fatalf(resultsNewErrFmt, err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+resultsPagePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf(routerCacheControlFmt, got, noStoreHeader)
	}
	contentType := rec.Header().Get(contentTypeHeader)
	if !strings.Contains(contentType, routerContentTypeHTML) {
		t.Fatalf(routerContentTypeWantFmt, contentType, routerContentTypeHTML)
	}
}

func TestResultsPageRouteRejectsInvalidID(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())

	store, err := results.New(t.TempDir()+resultsDBPath, 10)
	if err != nil {
		t.Fatalf(resultsNewErrFmt, err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/results/not-valid-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestUnknownAPIRouteReturnsJSONNotFound(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, config.DefaultConfig())
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+apiUnknownPath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
	contentType := rec.Header().Get(contentTypeHeader)
	if !strings.Contains(contentType, routerContentTypeJSON) {
		t.Fatalf(routerContentTypeWantFmt, contentType, routerContentTypeJSON)
	}
	if !strings.Contains(rec.Body.String(), `"error":"not found"`) {
		t.Fatalf(routerBodyNotFoundFmt, rec.Body.String())
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

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+registryHealthAPI, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFirstRegistryReq+statusWantFmt, rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+registryHealthAPI, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(routerSecondRegistryReq+statusWantFmt, rec.Code, http.StatusTooManyRequests)
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

	store, err := results.New(t.TempDir()+resultsDBPath, 10)
	if err != nil {
		t.Fatalf(resultsNewErrFmt, err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+resultsPagePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFirstResultsReq+statusWantFmt, rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+resultsPagePath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(routerSecondResultsReq+statusWantFmt, rec.Code, http.StatusTooManyRequests)
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

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+"/skill.html", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerSkillServedFmt, rec.Code)
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
