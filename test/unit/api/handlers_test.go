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

	contentTypeHeader              = "Content-Type"
	applicationJSON                = "application/json"
	textPlain                      = "text/plain"
	marshalPayloadErr              = "marshal payload: %v"
	statusCodeWantFmt              = "status = %d, want %d"
	handlersProtocolKey            = "protocol"
	handlersDirectionKey           = "direction"
	handlersDurationKey            = "duration"
	handlersStreamsKey             = "streams"
	handlersProtocolTCP            = "tcp"
	handlersDirDownload            = "download"
	handlersStreamIDKey            = "stream_id"
	handlersModeKey                = "mode"
	handlersPacketSizeKey          = "packet_size"
	handlersInvalidValue           = "invalid"
	handlersSidewaysValue          = "sideways"
	handlersClientMode             = "client"
	handlersPaddingKey             = "padding"
	handlersUnknownFieldJSON       = `{"protocol":"tcp","direction":"download","duration":10,"streams":1,"unknown":1}`
	handlersExpectedBodyDrainedErr = "expected request body to be drained"
	handlersExpectedBodyClosedErr  = "expected request body to be closed"
	handlersTestServerTCPKey       = "test_server_tcp"
	handlersTestServerUDPKey       = "test_server_udp"
	handlersTestServerTCPAddr      = "127.0.0.1:8081"
	handlersTestServerUDPAddr      = "127.0.0.1:8082"
	handlersCreateStreamStatusFmt  = "createTestStream: status = %d, body: %s"
	handlersDecodeStartStreamFmt   = "decode start stream response: %v"
	handlersStreamIDInvalidFmt     = "stream_id missing or invalid: %#v"
	handlersDurationWithinMaxFmt   = "duration within max: status = %d, want %d"
	handlersDurationBeyondMaxFmt   = "duration beyond max: status = %d, want %d"
	handlersMaxStreams32Fmt        = "32 streams with MaxStreams=32: status = %d, want %d; body: %s"
	handlersMaxStreams33Fmt        = "33 streams with MaxStreams=32: status = %d, want %d"
	handlersInvalidProtocolFmt     = "invalid protocol: status = %d, want %d"
	handlersInvalidDirectionFmt    = "invalid direction: status = %d, want %d"
	handlersInvalidModeFmt         = "invalid mode: status = %d, want %d"
	handlersSmallPacketFmt         = "small packet: status = %d, want %d"
	handlersFirstStreamStatusFmt   = "first stream status = %d, want %d"
	handlersSecondStreamStatusFmt  = "second stream status = %d, want %d; body: %s"
	handlersDecodeResponseFmt      = "decode response: %v"
	handlersTestServerTCPFmt       = "test_server_tcp = %#v, want %q"
	handlersTestServerUDPFmt       = "test_server_udp = %#v, want %q"
)

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatalf(marshalPayloadErr, err)
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
		t.Fatalf(handlersExpectedBodyDrainedErr)
	}
	if !tb.closed {
		t.Fatalf(handlersExpectedBodyClosedErr)
	}
}

// createTestStream creates a running stream and returns its ID.
func createTestStream(t *testing.T, handler *api.Handler) string {
	t.Helper()
	payload := map[string]any{
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf(handlersCreateStreamStatusFmt, rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf(handlersDecodeStartStreamFmt, err)
	}
	streamID, ok := resp[handlersStreamIDKey].(string)
	if !ok || streamID == "" {
		t.Fatalf(handlersStreamIDInvalidFmt, resp[handlersStreamIDKey])
	}
	return streamID
}

func TestStartStreamRejectsLargeBody(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := api.NewHandler(manager)

	payload := map[string]any{
		handlersProtocolKey:  handlersProtocolTCP,
		handlersDirectionKey: handlersDirDownload,
		handlersDurationKey:  10,
		handlersStreamsKey:   1,
		handlersPaddingKey:   strings.Repeat("a", (1<<20)+256),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf(marshalPayloadErr, err)
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
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1,
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
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1,
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

	req := httptest.NewRequest(http.MethodPost, streamStartPath, strings.NewReader(handlersUnknownFieldJSON))
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
			handlersProtocolKey:  handlersProtocolTCP,
			handlersDirectionKey: handlersDirDownload,
			handlersDurationKey:  10,
			handlersStreamsKey:   1,
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
		t.Fatalf(handlersExpectedBodyDrainedErr)
	}
	if !tb.closed {
		t.Fatalf(handlersExpectedBodyClosedErr)
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
		handlersProtocolKey:  handlersProtocolTCP,
		handlersDirectionKey: handlersDirDownload,
		handlersDurationKey:  50,
		handlersStreamsKey:   1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(handlersDurationWithinMaxFmt, rec.Code, http.StatusCreated)
	}

	// Duration exceeding the configured max should be rejected
	payload[handlersDurationKey] = 120
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersDurationBeyondMaxFmt, rec.Code, http.StatusBadRequest)
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
		handlersProtocolKey:  handlersProtocolTCP,
		handlersDirectionKey: handlersDirDownload,
		handlersDurationKey:  10,
		handlersStreamsKey:   32,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf(handlersMaxStreams32Fmt, rec.Code, http.StatusCreated, rec.Body.String())
	}

	// 33 streams should be rejected
	payload[handlersStreamsKey] = 33
	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersMaxStreams33Fmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidProtocol(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		handlersProtocolKey: "invalid", handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersInvalidProtocolFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidDirection(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersSidewaysValue,
		handlersDurationKey: 10, handlersStreamsKey: 1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersInvalidDirectionFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidMode(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1, handlersModeKey: handlersInvalidValue,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersInvalidModeFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamInvalidPacketSize(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	payload := map[string]any{
		handlersProtocolKey: handlersProtocolTCP, handlersDirectionKey: handlersDirDownload,
		handlersDurationKey: 10, handlersStreamsKey: 1, handlersPacketSizeKey: 10,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(handlersSmallPacketFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestStartStreamReturnsServiceUnavailableWhenAtCapacity(t *testing.T) {
	mgr := stream.NewManager(1, 0)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	payload := map[string]any{
		handlersProtocolKey:  handlersProtocolTCP,
		handlersDirectionKey: handlersDirDownload,
		handlersDurationKey:  10,
		handlersStreamsKey:   1,
	}

	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf(handlersFirstStreamStatusFmt, rec.Code, http.StatusCreated)
	}

	body = mustMarshalJSON(t, payload)
	req = httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec = httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf(handlersSecondStreamStatusFmt, rec.Code, http.StatusServiceUnavailable, rec.Body.String())
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
		handlersProtocolKey:  handlersProtocolTCP,
		handlersDirectionKey: handlersDirDownload,
		handlersDurationKey:  10,
		handlersStreamsKey:   1,
		handlersModeKey:      handlersClientMode,
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
		t.Fatalf(handlersDecodeResponseFmt, err)
	}
	if got, ok := resp[handlersTestServerTCPKey].(string); !ok || got != handlersTestServerTCPAddr {
		t.Fatalf(handlersTestServerTCPFmt, resp[handlersTestServerTCPKey], handlersTestServerTCPAddr)
	}
	if got, ok := resp[handlersTestServerUDPKey].(string); !ok || got != handlersTestServerUDPAddr {
		t.Fatalf(handlersTestServerUDPFmt, resp[handlersTestServerUDPKey], handlersTestServerUDPAddr)
	}
}
