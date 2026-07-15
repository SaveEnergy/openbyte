package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
)

func TestStaticHTMLUsesNoStoreCacheControl(t *testing.T) {
	router := api.NewRouter(config.DefaultConfig(), nil)

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf(routerCacheRootFmt, got, noStoreHeader)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+"/results.html", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf(routerCacheHTMLFmt, got, noStoreHeader)
	}
}

func TestStaticJSDoesNotForceNoStore(t *testing.T) {
	router := api.NewRouter(config.DefaultConfig(), nil)

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
	if !strings.Contains(csp, "worker-src 'self'") {
		t.Fatalf("csp missing worker-src self: %q", csp)
	}
	if !strings.Contains(csp, "connect-src 'self' https: http:") {
		t.Fatalf(routerCSPConnectSrcFmt, csp)
	}
	if strings.Contains(csp, "connect-src *") {
		t.Fatalf(routerCSPWildcardErrFmt, csp)
	}
}

func TestRateLimitSkipPaths(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	store, err := results.New(t.TempDir()+resultsDBPath, 10)
	if err != nil {
		t.Fatalf(resultsNewErrFmt, err)
	}
	defer store.Close()
	router := api.NewRouter(cfg, store)
	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+resultsPagePath, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(routerFirstResultsReq+statusWantFmt, rec.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+pingAPIPath, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal(routerPingBypassErr)
	}

	req = httptest.NewRequest(http.MethodGet, exampleBaseURL+downloadAPIPath+"?duration=0", nil)
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

}
