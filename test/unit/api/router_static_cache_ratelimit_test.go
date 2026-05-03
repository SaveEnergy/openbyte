package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

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
		t.Fatal(routerJSNoStoreErr)
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
		t.Fatal(routerCSPHeaderMissingErr)
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
		t.Fatal(routerPingBypassErr)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+downloadAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal(routerDownloadBypassErr)
	}

	req = httptest.NewRequest(http.MethodPost, exampleBaseURL+uploadAPIPath, strings.NewReader("x"))
	req.Header.Set(routerContentTypeKey, routerOctetStreamType)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal(routerUploadBypassErr)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+streamWSAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf(routerStreamRateLimitFmt, rec.Code)
	}
}
