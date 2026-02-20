package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

const (
	streamStartPath  = "/api/v1/stream/start"
	streamPathPrefix = "/api/v1/stream/"
	statusSuffix     = "/status"
	completeSuffix   = "/complete"
	metricsSuffix    = "/metrics"
	cancelSuffix     = "/cancel"
	resultsSuffix    = "/results"

	contentTypeHeader = "Content-Type"
	applicationJSON   = "application/json"
	textPlain         = "text/plain"
	statusCodeWantFmt = "status = %d, want %d"
)

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return body
}

type trackingBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (tb *trackingBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.offset >= len(tb.data) {
		return 0, io.EOF
	}
	n := copy(p, tb.data[tb.offset:])
	tb.offset += n
	return n, nil
}

func (tb *trackingBody) Close() error {
	tb.closed = true
	return nil
}

func assertTrackingBodyDrained(t *testing.T, tb *trackingBody) {
	t.Helper()
	if tb.reads == 0 {
		t.Fatalf("expected request body to be drained")
	}
	if !tb.closed {
		t.Fatalf("expected request body to be closed")
	}
}

// createTestStream creates a running stream and returns its ID.
func createTestStream(t *testing.T, handler *api.Handler) string {
	t.Helper()
	payload := map[string]any{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestStream: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode start stream response: %v", err)
	}
	streamID, ok := resp["stream_id"].(string)
	if !ok || streamID == "" {
		t.Fatalf("stream_id missing or invalid: %#v", resp["stream_id"])
	}
	return streamID
}

func TestStartStreamRejectsLargeBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]any{
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

	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestStartStreamRejectsWrongContentType(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]any{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, textPlain)
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestStartStreamRequiresContentType(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]any{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestStartStreamRejectsUnknownFields(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	req := httptest.NewRequest(http.MethodPost, streamStartPath, strings.NewReader(`{"protocol":"tcp","direction":"download","duration":10,"streams":1,"unknown":1}`))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamRejectsWrongContentTypeDrainsBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	tb := &trackingBody{
		data: mustMarshalJSON(t, map[string]any{
			"protocol":  "tcp",
			"direction": "download",
			"duration":  10,
			"streams":   1,
		}),
	}
	req := httptest.NewRequest(http.MethodPost, streamStartPath, nil)
	req.Body = tb
	req.Header.Set(contentTypeHeader, textPlain)
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
	if tb.reads == 0 {
		t.Fatalf("expected request body to be drained")
	}
	if !tb.closed {
		t.Fatalf("expected request body to be closed")
	}
}

func TestStartStreamRespectsMaxTestDuration(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 60 * time.Second
	handler.SetConfig(cfg)

	// Duration within the configured max should succeed
	payload := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  50,
		"streams":   1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("duration within max: status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// Duration exceeding the configured max should be rejected
	payload["duration"] = 120
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
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
	payload := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   32,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("32 streams with MaxStreams=32: status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// 33 streams should be rejected
	payload["streams"] = 33
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("33 streams with MaxStreams=32: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidProtocol(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		"protocol": "invalid", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid protocol: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidDirection(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		"protocol": "tcp", "direction": "sideways",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid direction: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidMode(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1, "mode": "invalid",
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidPacketSize(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1, "packet_size": 10,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("small packet: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamReturnsServiceUnavailableWhenAtCapacity(t *testing.T) {
	mgr := stream.NewManager(1, 0)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	payload := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
	}

	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first stream status = %d, want %d", rec.Code, http.StatusCreated)
	}

	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("second stream status = %d, want %d; body: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestStartStreamClientModeIgnoresRequestHostWhenProxyUntrusted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.TrustProxyHeaders = false
	cfg.TCPTestPort = 8081
	cfg.UDPTestPort = 8082
	handler.SetConfig(cfg)

	payload := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
		"mode":      "client",
	}

	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	req.Host = "evil.example:8888"
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusCreated)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, ok := resp["test_server_tcp"].(string); !ok || got != "127.0.0.1:8081" {
		t.Fatalf("test_server_tcp = %#v, want %q", resp["test_server_tcp"], "127.0.0.1:8081")
	}
	if got, ok := resp["test_server_udp"].(string); !ok || got != "127.0.0.1:8082" {
		t.Fatalf("test_server_udp = %#v, want %q", resp["test_server_udp"], "127.0.0.1:8082")
	}
}
