package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestCompleteStream(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{
		"status":  "completed",
		"metrics": map[string]any{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestCompleteStreamRejectsWrongContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{
		"status":  "completed",
		"metrics": map[string]any{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, textPlain)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCompleteStreamRequiresContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{
		"status":  "completed",
		"metrics": map[string]any{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCompleteStreamRejectsUnknownFields(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, strings.NewReader(`{"status":"completed","metrics":{"throughput_mbps":100},"unknown":1}`))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestCompleteStreamEarlyRejectsDrainBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	tests := []struct {
		name        string
		method      string
		contentType string
		wantStatus  int
	}{
		{name: "wrong method", method: http.MethodGet, contentType: applicationJSON, wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong content type", method: http.MethodPost, contentType: textPlain, wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &trackingBody{data: []byte(`{"status":"completed","metrics":{"throughput_mbps":1}}`)}
			req := httptest.NewRequest(tt.method, streamPathPrefix+"any"+completeSuffix, nil)
			req.Body = tb
			if tt.contentType != "" {
				req.Header.Set(contentTypeHeader, tt.contentType)
			}
			rec := httptest.NewRecorder()

			handler.CompleteStream(rec, req, "any")

			if rec.Code != tt.wantStatus {
				t.Fatalf(statusCodeWantFmt, rec.Code, tt.wantStatus)
			}
			assertTrackingBodyDrained(t, tb)
		})
	}
}

func TestCompleteStreamFailed(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{
		"status":  "failed",
		"metrics": map[string]any{},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
}

func TestCompleteStreamFailedStoresReason(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	const failureReason = "client aborted during upload"
	payload := map[string]any{
		"status":  "failed",
		"metrics": map[string]any{},
		"error":   failureReason,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	state, err := mgr.GetStream(streamID)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	snapshot := state.GetState()
	if snapshot.Error == nil {
		t.Fatal("stream error = nil, want stored failure reason")
	}
	if snapshot.Error.Error() != failureReason {
		t.Fatalf("stream error = %q, want %q", snapshot.Error.Error(), failureReason)
	}
}

func TestCompleteStreamInvalidStatus(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{"status": "invalid"}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status: code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCompleteStreamRejectsInvalidMetrics(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]any{
		"status": "completed",
		"metrics": map[string]any{
			"throughput_mbps": -1,
		},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
}
