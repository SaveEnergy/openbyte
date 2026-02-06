package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestStartStreamRejectsLargeBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
		"padding":   strings.Repeat("a", (1<<20)+256),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestStartStreamRespectsMaxTestDuration(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 60 * time.Second
	handler.SetConfig(cfg)

	// Duration within the configured max should succeed
	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  50,
		"streams":   1,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("duration within max: status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// Duration exceeding the configured max should be rejected
	payload["duration"] = 120
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duration beyond max: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamRespectsMaxStreams(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.MaxStreams = 32
	handler.SetConfig(cfg)

	// 32 streams should succeed
	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   32,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("32 streams with MaxStreams=32: status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// 33 streams should be rejected
	payload["streams"] = 33
	body, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("33 streams with MaxStreams=32: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
