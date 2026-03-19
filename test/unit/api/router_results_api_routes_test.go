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
