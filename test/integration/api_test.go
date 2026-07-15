package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
)

const (
	integrationJSONType  = "application/json"
	integrationOrigin    = "https://example.com"
	integrationStatusFmt = "Status code = %d, want %d"
	resultsPath          = "/api/v1/results"
	noStoreValue         = "no-store"
	resultsDBSuffix      = "/results.db"
)

func testConfig() *config.Config {
	return config.DefaultConfig()
}

func mustStringField(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("response missing %s", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		t.Fatalf("response %s invalid type/value: %#v", key, v)
	}
	return s
}

func TestCrossOriginAccessIsLimitedToPing(t *testing.T) {
	handler := api.NewRouter(testConfig(), nil).SetupRoutes()
	tests := []struct {
		path       string
		wantStatus int
		wantOrigin string
	}{
		{path: "/api/v1/ping", wantStatus: http.StatusOK, wantOrigin: "*"},
		{path: "/health", wantStatus: http.StatusOK},
		{path: "/api/v1/nonexistent", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		req.Header.Set("Origin", integrationOrigin)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != tt.wantStatus {
			t.Errorf("%s: "+integrationStatusFmt, tt.path, rec.Code, tt.wantStatus)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
			t.Errorf("%s: Access-Control-Allow-Origin = %q, want %q", tt.path, got, tt.wantOrigin)
		}
	}
}

func TestAPIResultsSaveAndGet(t *testing.T) {
	store, err := results.New(t.TempDir()+resultsDBSuffix, 100)
	if err != nil {
		t.Fatalf("results.New: %v", err)
	}
	defer store.Close()
	router := api.NewRouter(testConfig(), store)

	h := router.SetupRoutes()

	saveBody := `{
		"download_mbps": 321.5,
		"upload_mbps": 123.4,
		"latency_ms": 11.2,
		"jitter_ms": 1.5,
		"loaded_latency_ms": 16.8,
		"bufferbloat_grade": "A",
		"ipv4": "203.0.113.10",
		"ipv6": "",
		"server_name": "integration-server"
	}`
	saveReq := httptest.NewRequest(http.MethodPost, resultsPath, bytes.NewBufferString(saveBody))
	saveReq.Header.Set("Content-Type", integrationJSONType)
	saveRec := httptest.NewRecorder()
	h.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusCreated {
		t.Fatalf("save status = %d, want %d; body=%s", saveRec.Code, http.StatusCreated, saveRec.Body.String())
	}

	var saveResp map[string]any
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	id := mustStringField(t, saveResp, "id")

	getReq := httptest.NewRequest(http.MethodGet, resultsPath+"/"+id, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	if got := getRec.Header().Get("Cache-Control"); got != noStoreValue {
		t.Fatalf("cache-control = %q, want %q", got, noStoreValue)
	}

	var gotResult map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &gotResult); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if gotID, ok := gotResult["id"].(string); !ok || gotID != id {
		t.Fatalf("result id = %#v, want %q", gotResult["id"], id)
	}
}
