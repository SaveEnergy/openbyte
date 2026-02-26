package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	integrationGetMethod   = "GET"
	integrationPostMethod  = "POST"
	integrationOptionsVerb = "OPTIONS"
	integrationJSONType    = "application/json"
	integrationOrigin      = "https://example.com"
	integrationStatusFmt   = "Status code = %d, want %d"
	resultsPath            = "/api/v1/results"
	noStoreValue           = "no-store"
	resultsDBSuffix        = "/results.db"
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

func TestAPIStartStream(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)

	reqBody := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   4,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(integrationPostMethod, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", integrationJSONType)
	w := httptest.NewRecorder()

	handler.StartStream(w, req)

	if w.Code != http.StatusCreated {
		t.Logf("Response body: %s", w.Body.String())
		t.Errorf(integrationStatusFmt, w.Code, http.StatusCreated)
		return
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	_ = mustStringField(t, resp, "stream_id")
	_ = mustStringField(t, resp, "websocket_url")
}

func TestAPIGetStreamStatus(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)

	config := types.StreamConfig{
		Protocol:  types.ProtocolTCP,
		Direction: types.DirectionDownload,
		Duration:  10 * time.Second,
		Streams:   4,
	}
	state, err := manager.CreateStream(config)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	req := httptest.NewRequest(integrationGetMethod, "/api/v1/stream/"+state.Config.ID+"/status", nil)
	w := httptest.NewRecorder()

	router := api.NewRouter(handler, testConfig())
	router.SetupRoutes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf(integrationStatusFmt, w.Code, http.StatusOK)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())
	router.SetAllowedOrigins([]string{integrationOrigin})

	req := httptest.NewRequest(integrationGetMethod, "/health", nil)
	req.Header.Set("Origin", integrationOrigin)
	w := httptest.NewRecorder()

	router.SetupRoutes().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != integrationOrigin {
		t.Errorf("Access-Control-Allow-Origin = %s, want %s", w.Header().Get("Access-Control-Allow-Origin"), integrationOrigin)
	}
}

func TestCORSBlockedOrigin(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())
	router.SetAllowedOrigins([]string{integrationOrigin})

	req := httptest.NewRequest(integrationOptionsVerb, "/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()

	router.SetupRoutes().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf(integrationStatusFmt, w.Code, http.StatusForbidden)
	}
}

func TestAPIResultsSaveAndGet(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())

	store, err := results.New(t.TempDir()+resultsDBSuffix, 100)
	if err != nil {
		t.Fatalf("results.New: %v", err)
	}
	defer store.Close()
	router.SetResultsHandler(results.NewHandler(store))

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
