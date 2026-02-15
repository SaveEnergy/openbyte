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
	"github.com/saveenergy/openbyte/pkg/types"
)

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
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
	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestStream: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
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
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestStartStreamRejectsWrongContentType(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestStartStreamRequiresContentType(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestStartStreamRejectsUnknownFields(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", strings.NewReader(`{"protocol":"tcp","direction":"download","duration":10,"streams":1,"unknown":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamRejectsWrongContentTypeDrainsBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	tb := &trackingBody{
		data: mustMarshalJSON(t, map[string]interface{}{
			"protocol":  "tcp",
			"direction": "download",
			"duration":  10,
			"streams":   1,
		}),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", nil)
	req.Body = tb
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.StartStream(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
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
	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  50,
		"streams":   1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("duration within max: status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// Duration exceeding the configured max should be rejected
	payload["duration"] = 120
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("32 streams with MaxStreams=32: status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	// 33 streams should be rejected
	payload["streams"] = 33
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("33 streams with MaxStreams=32: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetVersion(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	handler.SetVersion("1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", resp.Version)
	}
}

func TestGetVersionDefault(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode default version response: %v", err)
	}
	if resp.Version != "dev" {
		t.Errorf("default version = %q, want dev", resp.Version)
	}
}

func TestGetVersionDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	req.Body = tb
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetServers(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.ServerName = "Test Server"
	handler.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Host = "testhost:8080"
	rec := httptest.NewRecorder()
	handler.GetServers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp api.ServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode servers response: %v", err)
	}
	if len(resp.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(resp.Servers))
	}
	if resp.Servers[0].Name != "Test Server" {
		t.Errorf("server name = %q, want Test Server", resp.Servers[0].Name)
	}
	if resp.Servers[0].Health != "healthy" {
		t.Errorf("health = %q, want healthy", resp.Servers[0].Health)
	}
}

func TestGetServersIgnoresRequestHostWhenProxyUntrusted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = "8080"
	cfg.TrustProxyHeaders = false
	handler.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Host = "evil.example:9999"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.GetServers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp api.ServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode servers response: %v", err)
	}
	if len(resp.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(resp.Servers))
	}
	if resp.Servers[0].APIEndpoint != "http://127.0.0.1:8080" {
		t.Fatalf("api endpoint = %q, want %q", resp.Servers[0].APIEndpoint, "http://127.0.0.1:8080")
	}
}

func TestGetServersRejectsWrongMethodDrainsBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetServers(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetStreamStatus(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+streamID+"/status", nil)
	req.SetPathValue("id", streamID)
	rec := httptest.NewRecorder()
	handler.GetStreamStatus(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestGetStreamStatusDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+streamID+"/status", nil)
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
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestReportMetrics(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	metrics := types.Metrics{ThroughputMbps: 500, BytesTransferred: 1024}
	body := mustMarshalJSON(t, metrics)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

func TestReportMetricsRejectsWrongContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	body := mustMarshalJSON(t, types.Metrics{ThroughputMbps: 500})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestReportMetricsRequiresContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	body := mustMarshalJSON(t, types.Metrics{ThroughputMbps: 500})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/metrics", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestReportMetricsRejectsUnknownFields(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/metrics", strings.NewReader(`{"throughput_mbps":100,"unknown":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReportMetricsValidation(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	metrics := types.Metrics{
		ThroughputMbps: -1,
	}
	body := mustMarshalJSON(t, metrics)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReportMetricsEarlyRejectsDrainBody(t *testing.T) {
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
		{name: "wrong method", method: http.MethodGet, contentType: "application/json", wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong content type", method: http.MethodPost, contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &trackingBody{data: []byte(`{"throughput_mbps":123}`)}
			req := httptest.NewRequest(tt.method, "/api/v1/stream/any/metrics", nil)
			req.Body = tb
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			handler.ReportMetrics(rec, req, "any")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			assertTrackingBodyDrained(t, tb)
		})
	}
}

func TestReportMetricsNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	body := mustMarshalJSON(t, types.Metrics{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/missing/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestCompleteStream(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]interface{}{
		"status":  "completed",
		"metrics": map[string]interface{}{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

	payload := map[string]interface{}{
		"status":  "completed",
		"metrics": map[string]interface{}{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCompleteStreamRequiresContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]interface{}{
		"status":  "completed",
		"metrics": map[string]interface{}{"throughput_mbps": 100},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestCompleteStreamRejectsUnknownFields(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", strings.NewReader(`{"status":"completed","metrics":{"throughput_mbps":100},"unknown":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
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
		{name: "wrong method", method: http.MethodGet, contentType: "application/json", wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong content type", method: http.MethodPost, contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &trackingBody{data: []byte(`{"status":"completed","metrics":{"throughput_mbps":1}}`)}
			req := httptest.NewRequest(tt.method, "/api/v1/stream/any/complete", nil)
			req.Body = tb
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			handler.CompleteStream(rec, req, "any")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
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

	payload := map[string]interface{}{
		"status":  "failed",
		"metrics": map[string]interface{}{},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCompleteStreamInvalidStatus(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	payload := map[string]interface{}{"status": "invalid"}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status: code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCancelStream(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/cancel", nil)
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCancelStreamWithBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/cancel", bytes.NewReader([]byte(`{"reason":"user"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCancelStreamNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/missing/cancel", nil)
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetStreamResultsNotCompleted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+streamID+"/results", nil)
	rec := httptest.NewRecorder()
	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (not completed yet)", rec.Code, http.StatusAccepted)
	}
}

func TestGetStreamResultsDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+streamID+"/results", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetStreamResultsCompleted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	// Complete it first
	payload := map[string]interface{}{
		"status":  "completed",
		"metrics": map[string]interface{}{"throughput_mbps": 250},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/"+streamID+"/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	// Now get results
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+streamID+"/results", nil)
	rec = httptest.NewRecorder()
	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestStartStreamInvalidProtocol(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]interface{}{
		"protocol": "invalid", "direction": "download",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid protocol: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidDirection(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "sideways",
		"duration": 10, "streams": 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid direction: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidMode(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1, "mode": "invalid",
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidPacketSize(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]interface{}{
		"protocol": "tcp", "direction": "download",
		"duration": 10, "streams": 1, "packet_size": 10,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
	}

	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first stream status = %d, want %d", rec.Code, http.StatusCreated)
	}

	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

	payload := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  10,
		"streams":   1,
		"mode":      "client",
	}

	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Host = "evil.example:8888"
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
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
