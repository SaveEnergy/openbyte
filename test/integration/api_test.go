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

func testConfig() *config.Config {
	return config.DefaultConfig()
}

func mustStringField(t *testing.T, m map[string]interface{}, key string) string {
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

func TestAPI_StartStream(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   4,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.StartStream(w, req)

	if w.Code != http.StatusCreated {
		t.Logf("Response body: %s", w.Body.String())
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusCreated)
		return
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	_ = mustStringField(t, resp, "stream_id")
	_ = mustStringField(t, resp, "websocket_url")
}

func TestAPI_GetStreamStatus(t *testing.T) {
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

	req := httptest.NewRequest("GET", "/api/v1/stream/"+state.Config.ID+"/status", nil)
	w := httptest.NewRecorder()

	router := api.NewRouter(handler, testConfig())
	router.SetupRoutes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())
	router.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	router.SetupRoutes().ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %s, want %s", w.Header().Get("Access-Control-Allow-Origin"), "https://example.com")
	}
}

func TestCORSBlockedOrigin(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())
	router.SetAllowedOrigins([]string{"https://example.com"})

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()

	router.SetupRoutes().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAPI_ResultsSaveAndGet(t *testing.T) {
	manager := stream.NewManager(10, 2)
	manager.Start()
	defer manager.Stop()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, testConfig())

	store, err := results.New(t.TempDir()+"/results.db", 100)
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
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/results", bytes.NewBufferString(saveBody))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	h.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusCreated {
		t.Fatalf("save status = %d, want %d; body=%s", saveRec.Code, http.StatusCreated, saveRec.Body.String())
	}

	var saveResp map[string]interface{}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	id := mustStringField(t, saveResp, "id")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/results/"+id, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	if got := getRec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}

	var gotResult map[string]interface{}
	if err := json.Unmarshal(getRec.Body.Bytes(), &gotResult); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if gotID, ok := gotResult["id"].(string); !ok || gotID != id {
		t.Fatalf("result id = %#v, want %q", gotResult["id"], id)
	}
}
