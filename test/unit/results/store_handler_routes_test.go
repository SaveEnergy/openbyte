package results_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/results"
)

func TestHandlerSaveValidation(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc(resultsPostRoute, handler.Save)
	router.HandleFunc(resultsGetRoute, handler.Get)

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"valid", `{"download_mbps":100,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusCreated},
		{"negative download", `{"download_mbps":-1,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"out of range", `{"download_mbps":200000,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`, http.StatusBadRequest},
		{"multiple json objects", `{"download_mbps":100}{"upload_mbps":50}`, http.StatusBadRequest},
		{"invalid json", `{bad}`, http.StatusBadRequest},
		{"empty body", ``, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(storeHTTPMethodPost, resultsAPIPath, strings.NewReader(tt.body))
			req.Header.Set(storeContentTypeHeader, jsonContentType)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf(storeStatusWantFmt+"; body: %s", rec.Code, tt.status, rec.Body.String())
			}
		})
	}
}

func TestHandlerGetInvalidID(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc(resultsGetRoute, handler.Get)

	tests := []struct {
		name   string
		id     string
		status int
	}{
		{"too short", "abc", http.StatusBadRequest},
		{"too long", "abcdefgh1234", http.StatusBadRequest},
		{"special chars", "abc-_...", http.StatusBadRequest},
		{"valid but missing", "abcd1234", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(storeHTTPMethodGet, resultsAPIBasePath+tt.id, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Errorf(storeStatusWantFmt, rec.Code, tt.status)
			}
		})
	}
}

func TestHandlerSaveRejectsWrongContentType(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc(resultsPostRoute, handler.Save)

	body := `{"download_mbps":100,"upload_mbps":50,"latency_ms":10,"jitter_ms":1}`
	req := httptest.NewRequest(storeHTTPMethodPost, resultsAPIPath, strings.NewReader(body))
	req.Header.Set(storeContentTypeHeader, storeTextPlainContentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf(storeTextPlainStatusFmt, rec.Code, http.StatusUnsupportedMediaType)
	}

	// Verify the error response is JSON
	var errResp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Errorf(storeErrorResponseJSONFmt, err)
	}
	if errResp["error"] == "" {
		t.Error(storeExpectedErrorField)
	}
}

func TestHandlerRoundTrip(t *testing.T) {
	store, cleanup := tempStore(t, 100)
	defer cleanup()

	handler := results.NewHandler(store)
	router := http.NewServeMux()
	router.HandleFunc(resultsPostRoute, handler.Save)
	router.HandleFunc(resultsGetRoute, handler.Get)

	// Save
	body := `{"download_mbps":500.5,"upload_mbps":100.2,"latency_ms":8.1,"jitter_ms":0.5,"loaded_latency_ms":15.3,"bufferbloat_grade":"B","ipv4":"203.0.113.1","ipv6":"2001:db8::1","server_name":"Test"}`
	req := httptest.NewRequest(storeHTTPMethodPost, resultsAPIPath, strings.NewReader(body))
	req.Header.Set(storeContentTypeHeader, jsonContentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(storeStatusWithBodyFmt, storeSaveAction, rec.Code, http.StatusCreated, rec.Body.String())
	}

	var saveResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&saveResp); err != nil {
		t.Fatalf(storeDecodeResponseFmt, storeSaveAction, err)
	}
	if saveResp.ID == "" {
		t.Fatal(storeEmptyIDMsg)
	}
	if saveResp.URL != "/results/"+saveResp.ID {
		t.Errorf("url = %q, want /results/%s", saveResp.URL, saveResp.ID)
	}

	// Fetch
	req = httptest.NewRequest(storeHTTPMethodGet, resultsAPIBasePath+saveResp.ID, nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(storeStatusWithBodyFmt, storeGetAction, rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get(storeCacheControlHeader); got != storeCacheNoStore {
		t.Fatalf(storeCacheControlFmt, got, storeCacheNoStore)
	}

	var result results.Result
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf(storeDecodeResponseFmt, storeGetAction, err)
	}
	if result.DownloadMbps != 500.5 {
		t.Errorf("download_mbps = %v, want 500.5", result.DownloadMbps)
	}
	if result.ServerName != "Test" {
		t.Errorf("server_name = %q, want Test", result.ServerName)
	}
}
