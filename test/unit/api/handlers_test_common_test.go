package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
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
