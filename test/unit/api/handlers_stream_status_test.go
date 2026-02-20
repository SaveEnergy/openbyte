package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestGetStreamStatus(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	req.SetPathValue("id", streamID)
	rec := httptest.NewRecorder()
	handler.GetStreamStatus(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetStreamStatusJSONContract(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	payload := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  12,
		"streams":   1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start stream status = %d, want %d", rec.Code, http.StatusCreated)
	}
	var startResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&startResp); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	streamID, _ := startResp["stream_id"].(string)

	statusReq := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	statusRec := httptest.NewRecorder()
	handler.GetStreamStatus(statusRec, statusReq, streamID)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", statusRec.Code, http.StatusOK)
	}

	var statusResp map[string]any
	if err := json.NewDecoder(statusRec.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	cfg, ok := statusResp["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %T", statusResp["config"])
	}
	duration, ok := cfg["duration"].(float64)
	if !ok {
		t.Fatalf("config.duration missing or wrong type: %T", cfg["duration"])
	}
	if int(duration) != 12 {
		t.Fatalf("config.duration = %v, want %d seconds", duration, 12)
	}
}

func TestGetStreamStatusDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetStreamStatus(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetStreamStatusNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/nonexistent/status", nil)
	rec := httptest.NewRecorder()
	handler.GetStreamStatus(rec, req, "nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}
