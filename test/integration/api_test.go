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
