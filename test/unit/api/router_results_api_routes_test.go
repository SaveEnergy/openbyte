package api_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

func TestResultsPageUsesStaticGzipHandler(t *testing.T) {
	store := newTestResultsStore(t)
	router := api.NewRouter(config.DefaultConfig(), store)

	h := router.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+resultsPagePath, nil)
	req.Header.Set("Accept-Encoding", "gzip")
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
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("content-encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("vary = %q, want Accept-Encoding", got)
	}
	compressedLen := rec.Body.Len()
	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("open gzip response: %v", err)
	}
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip response: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip response: %v", err)
	}
	if !strings.Contains(string(decompressed), "<!doctype html>") {
		t.Fatal("gzip response did not contain results HTML")
	}

	headReq := httptest.NewRequest(http.MethodHead, exampleBaseURL+resultsPagePath, nil)
	headReq.Header.Set("Accept-Encoding", "gzip")
	headRec := httptest.NewRecorder()
	h.ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusOK {
		t.Fatalf("HEAD "+statusWantFmt, headRec.Code, http.StatusOK)
	}
	if headRec.Body.Len() != 0 {
		t.Fatalf("HEAD body length = %d, want 0", headRec.Body.Len())
	}
	if got, want := headRec.Header().Get("Content-Length"), strconv.Itoa(compressedLen); got != want {
		t.Fatalf("HEAD content-length = %q, want %q", got, want)
	}
	if got := headRec.Header().Get(cacheControlKey); got != noStoreHeader {
		t.Fatalf("HEAD "+routerCacheControlFmt, got, noStoreHeader)
	}
}

func TestResultsPageRouteRejectsInvalidID(t *testing.T) {
	store := newTestResultsStore(t)
	router := api.NewRouter(config.DefaultConfig(), store)

	h := router.SetupRoutes()
	req := httptest.NewRequest(http.MethodGet, exampleBaseURL+"/results/not-valid-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestUnknownAPIRouteReturnsJSONNotFound(t *testing.T) {
	router := api.NewRouter(config.DefaultConfig(), nil)
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

func TestResultsRoutesAbsentWithoutStore(t *testing.T) {
	h := api.NewRouter(config.DefaultConfig(), nil).SetupRoutes()
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/results"},
		{method: http.MethodGet, path: "/api/v1/results/abc12345"},
		{method: http.MethodGet, path: resultsPagePath},
		{method: http.MethodHead, path: resultsPagePath},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(test.method, exampleBaseURL+test.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf(statusWantFmt, rec.Code, http.StatusNotFound)
			}
		})
	}
}

func TestResultsAPIRoutesRateLimited(t *testing.T) {
	for _, test := range []struct {
		name        string
		method      string
		path        string
		body        string
		firstStatus int
	}{
		{name: "save", method: http.MethodPost, path: "/api/v1/results", body: `{}`, firstStatus: http.StatusCreated},
		{name: "get", method: http.MethodGet, path: "/api/v1/results/abc12345", firstStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.GlobalRateLimit = 1
			cfg.RateLimitPerIP = 1
			h := api.NewRouter(cfg, newTestResultsStore(t)).SetupRoutes()

			request := func() *http.Request {
				req := httptest.NewRequest(test.method, exampleBaseURL+test.path, strings.NewReader(test.body))
				if test.method == http.MethodPost {
					req.Header.Set(contentTypeHeader, routerContentTypeJSON)
				}
				return req
			}
			first := httptest.NewRecorder()
			h.ServeHTTP(first, request())
			if first.Code != test.firstStatus {
				t.Fatalf("first request "+statusWantFmt, first.Code, test.firstStatus)
			}

			second := httptest.NewRecorder()
			h.ServeHTTP(second, request())
			if second.Code != http.StatusTooManyRequests {
				t.Fatalf("second request "+statusWantFmt, second.Code, http.StatusTooManyRequests)
			}
			if got := second.Header().Get("Retry-After"); got != "60" {
				t.Fatalf("retry-after = %q, want 60", got)
			}
		})
	}
}

func TestResultsPageRouteRateLimited(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	store := newTestResultsStore(t)
	router := api.NewRouter(cfg, store)
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
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry-after = %q, want 60", got)
	}
	if got := rec.Header().Get(contentTypeHeader); !strings.Contains(got, routerContentTypeJSON) {
		t.Fatalf(routerContentTypeWantFmt, got, routerContentTypeJSON)
	}
	if !strings.Contains(rec.Body.String(), `"error":"rate limit exceeded"`) {
		t.Fatalf("rate-limit body = %q, want JSON error", rec.Body.String())
	}
}
