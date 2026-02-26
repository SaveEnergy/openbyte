package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

const (
	speedtestStatusFmt                 = "status = %d, want %d"
	octetStreamType                    = "application/octet-stream"
	jsonContentType                    = "application/json"
	noStoreCacheControl                = "no-store"
	downloadEndpointBase               = "http://example.com/api/v1/download"
	uploadEndpoint                     = "http://example.com/api/v1/upload"
	pingEndpoint                       = "http://example.com/api/v1/ping"
	statusServiceUnavailable           = http.StatusServiceUnavailable
	statusBadRequest                   = http.StatusBadRequest
	statusInternalServerErr            = http.StatusInternalServerError
	speedtestQueryDur1Chunk            = "?duration=1&chunk=65536"
	speedtestQueryDur60Chunk           = "?duration=60&chunk=65536"
	speedtestQueryDur5Chunk            = "?duration=5&chunk=65536"
	speedtestQueryDur10Chunk           = "?duration=10&chunk=65536"
	speedtestQueryDurZero              = "?duration=0"
	speedtestQueryChunkBad             = "?chunk=bad"
	speedtestQueryChunkABC             = "?chunk=abc"
	speedtestWaitTimeout               = 2 * time.Second
	pingClientIPKey                    = "client_ip"
	pingIPv6Key                        = "ipv6"
	pingClientIPv4Want                 = "203.0.113.10"
	pingClientIPv6Want                 = "2001:db8::1"
	pingIPv6FalseMsg                   = "ipv6 = %v, want false"
	pingIPv6TrueMsg                    = "ipv6 = %v, want true"
	speedtestContentTypeKey            = "Content-Type"
	speedtestCacheControlKey           = "Cache-Control"
	speedtestDecodeRespFmt             = "decode response: %v"
	speedtestExpectBodyDrained         = "expected body to be drained"
	speedtestExpectBodyClosed          = "expected body to be closed"
	speedtestExpectReqBodyDrained503   = "expected request body to be drained before returning 503"
	speedtestExpectReqBodyClosed       = "expected request body to be closed"
	speedtestWaitDownloadStartTimeout  = "timed out waiting for download goroutine to start"
	speedtestWaitUploadStartTimeout    = "timed out waiting for upload goroutine to start"
	speedtestExpected503AtLimitFmt     = "expected 503 when at limit, got %d"
	speedtestExpected200AfterCancelFmt = "expected 200 after cancel freed slots, got %d"
	speedtestURLStatusFmt              = "url=%s status = %d, want %d"
	speedtestURLBodyDrainedFmt         = "url=%s expected body to be drained"
	speedtestURLBodyClosedFmt          = "url=%s expected body to be closed"
	speedtestDurationTooHighFmt        = "duration=10 with max=5: status = %d, want 400"
	speedtestChunkABCFmt               = "chunk=abc: status = %d, want 400"
)

// signalWriter wraps httptest.ResponseRecorder and signals a channel on first Write,
// indicating the handler has started (and incremented its concurrency counter).
type signalWriter struct {
	*httptest.ResponseRecorder
	started chan struct{}
	once    sync.Once
}

func (sw *signalWriter) Write(b []byte) (int, error) {
	sw.once.Do(func() { close(sw.started) })
	return sw.ResponseRecorder.Write(b)
}

func (sw *signalWriter) WriteHeader(code int) {
	sw.once.Do(func() { close(sw.started) })
	sw.ResponseRecorder.WriteHeader(code)
}

// signalReader wraps a Reader and signals on the first Read call.
type signalReader struct {
	io.ReadCloser
	started chan struct{}
	once    sync.Once
}

func (sr *signalReader) Read(p []byte) (int, error) {
	sr.once.Do(func() { close(sr.started) })
	return sr.ReadCloser.Read(p)
}

func TestSpeedTestDownloadWritesData(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	t.Cleanup(cancel)

	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(speedtestContentTypeKey); got != octetStreamType {
		t.Fatalf("content-type = %q, want %q", got, octetStreamType)
	}
	if rec.Header().Get(speedtestCacheControlKey) == "" {
		t.Fatalf("cache-control header missing")
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-zero download body")
	}
}

func TestSpeedTestUploadReportsBytes(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	payload := bytes.Repeat([]byte("a"), 256*1024)
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader(payload))
	req.Header.Set(speedtestContentTypeKey, octetStreamType)
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}

	var resp struct {
		Bytes int64 `json:"bytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if resp.Bytes != int64(len(payload)) {
		t.Fatalf("bytes = %d, want %d", resp.Bytes, len(payload))
	}
}

type errReader struct {
	readOnce bool
}

func (e *errReader) Read(p []byte) (int, error) {
	if !e.readOnce {
		e.readOnce = true
		p[0] = 'x'
		return 1, nil
	}
	return 0, errors.New("read failure")
}

type trackingUploadBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (tb *trackingUploadBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.offset >= len(tb.data) {
		return 0, io.EOF
	}
	n := copy(p, tb.data[tb.offset:])
	tb.offset += n
	return n, nil
}

func (tb *trackingUploadBody) Close() error {
	tb.closed = true
	return nil
}

type failingTrackingBody struct {
	reads  int
	closed bool
}

func (tb *failingTrackingBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.reads == 1 {
		p[0] = 'x'
		return 1, nil
	}
	return 0, errors.New("read failure")
}

func (tb *failingTrackingBody) Close() error {
	tb.closed = true
	return nil
}

type blockingUploadBody struct {
	closed chan struct{}
	once   sync.Once
}

func (b *blockingUploadBody) Read(_ []byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingUploadBody) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return nil
}

func TestDownloadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all slots with long-running downloads using signal writers
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)
	started := make([]chan struct{}, maxConcurrent)

	for i := range maxConcurrent {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})

		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur60Chunk, nil)
			req = req.WithContext(ctx)
			sw := &signalWriter{ResponseRecorder: httptest.NewRecorder(), started: ch}
			handler.Download(sw, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitDownloadStartTimeout)
		}
	}

	// New download should get 503
	reqOver := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	recOver := httptest.NewRecorder()
	handler.Download(recOver, reqOver)
	if recOver.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, recOver.Code)
	}

	// Cancel all running downloads (simulates user pressing cancel)
	for _, cancel := range cancels {
		cancel()
	}
	for range maxConcurrent {
		<-done
	}

	// After cancellation, new download should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)
	reqAfter := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	reqAfter = reqAfter.WithContext(ctx)
	recAfter := httptest.NewRecorder()
	handler.Download(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(speedtestExpected200AfterCancelFmt, recAfter.Code)
	}
}

func TestDownloadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != statusServiceUnavailable {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusServiceUnavailable)
	}
	if tb.reads == 0 {
		t.Fatalf(speedtestExpectReqBodyDrained503)
	}
	if !tb.closed {
		t.Fatalf(speedtestExpectReqBodyClosed)
	}
}

func TestDownloadValidationRejectsDrainBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	tests := []string{
		downloadEndpointBase + speedtestQueryDurZero,
		downloadEndpointBase + speedtestQueryChunkBad,
	}
	for _, u := range tests {
		tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
		req := httptest.NewRequest(http.MethodGet, u, nil)
		req.Body = tb
		rec := httptest.NewRecorder()

		handler.Download(rec, req)

		if rec.Code != statusBadRequest {
			t.Fatalf(speedtestURLStatusFmt, u, rec.Code, statusBadRequest)
		}
		if tb.reads == 0 {
			t.Fatalf(speedtestURLBodyDrainedFmt, u)
		}
		if !tb.closed {
			t.Fatalf(speedtestURLBodyClosedFmt, u)
		}
	}
}

func TestSpeedTestUploadHandlesReadError(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = io.NopCloser(&errReader{})
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
}

func TestSpeedTestUploadReadErrorDrainsBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &failingTrackingBody{}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
	if tb.reads == 0 {
		t.Fatal("expected upload body to be read")
	}
	if !tb.closed {
		t.Fatal("expected upload body to be closed")
	}
}

func TestUploadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all upload slots with long-running uploads using signal readers
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)
	started := make([]chan struct{}, maxConcurrent)

	for i := range maxConcurrent {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})

		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			// Slow reader that blocks until context is cancelled
			pr, pw := io.Pipe()
			go func() {
				<-ctx.Done()
				pw.Close()
			}()
			body := &signalReader{ReadCloser: io.NopCloser(pr), started: ch}
			req := httptest.NewRequest(http.MethodPost, uploadEndpoint, body)
			rec := httptest.NewRecorder()
			handler.Upload(rec, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitUploadStartTimeout)
		}
	}

	// New upload should get 503
	reqOver := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader([]byte("data")))
	recOver := httptest.NewRecorder()
	handler.Upload(recOver, reqOver)
	if recOver.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, recOver.Code)
	}

	// Cancel all running uploads
	for _, cancel := range cancels {
		cancel()
	}
	for range maxConcurrent {
		<-done
	}

	// After cancellation, new upload should succeed
	reqAfter := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader(bytes.Repeat([]byte("x"), 1024)))
	recAfter := httptest.NewRecorder()
	handler.Upload(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(speedtestExpected200AfterCancelFmt, recAfter.Code)
	}
}

func TestUploadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != statusServiceUnavailable {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusServiceUnavailable)
	}
	if tb.reads == 0 {
		t.Fatalf(speedtestExpectReqBodyDrained503)
	}
	if !tb.closed {
		t.Fatalf(speedtestExpectReqBodyClosed)
	}
}

func TestUploadRespectsReadDeadlineWhenBodyStalls(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 1)
	body := &blockingUploadBody{closed: make(chan struct{})}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = body
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.Upload(rec, req)
	elapsed := time.Since(start)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
	if elapsed > speedtestWaitTimeout {
		t.Fatalf("upload read deadline not enforced, elapsed=%v", elapsed)
	}
}

func TestDownloadRespectsMaxDuration(t *testing.T) {
	// maxDurationSec=5: duration=5 should work, duration=10 should be clamped to default
	handler := api.NewSpeedTestHandler(10, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// duration=5 (within max) should be accepted
	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur5Chunk, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.Download(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duration=5 with max=5: status = %d, want 200", rec.Code)
	}

	// duration=10 (above max=5) should be rejected with 400
	req2 := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur10Chunk, nil)
	rec2 := httptest.NewRecorder()
	handler.Download(rec2, req2)
	if rec2.Code != statusBadRequest {
		t.Fatalf(speedtestDurationTooHighFmt, rec2.Code)
	}

	// invalid chunk should be rejected with 400
	req3 := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryChunkABC, nil)
	rec3 := httptest.NewRecorder()
	handler.Download(rec3, req3)
	if rec3.Code != statusBadRequest {
		t.Fatalf(speedtestChunkABCFmt, rec3.Code)
	}
}

func TestSpeedTestHandlerPingResponseShape(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(speedtestContentTypeKey); got != jsonContentType {
		t.Fatalf("content-type = %q, want %q", got, jsonContentType)
	}
	if got := rec.Header().Get(speedtestCacheControlKey); got != noStoreCacheControl {
		t.Fatalf("cache-control = %q, want %q", got, noStoreCacheControl)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if pong, ok := resp["pong"].(bool); !ok || !pong {
		t.Fatalf("pong = %v, want true", resp["pong"])
	}
	if _, ok := resp["timestamp"].(float64); !ok {
		t.Fatalf("timestamp missing or wrong type: %T", resp["timestamp"])
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv4Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv4Want)
	}
	if ipv6, ok := resp[pingIPv6Key].(bool); !ok || ipv6 {
		t.Fatalf(pingIPv6FalseMsg, resp[pingIPv6Key])
	}
}

func TestSpeedTestHandlerPingNilResolverFallback(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.RemoteAddr = "[2001:db8::1]:4242"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv6Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv6Want)
	}
	if ipv6, ok := resp[pingIPv6Key].(bool); !ok || !ipv6 {
		t.Fatalf(pingIPv6TrueMsg, resp[pingIPv6Key])
	}
}

func TestSpeedTestHandlerPingDrainsUnexpectedBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if tb.reads == 0 {
		t.Fatal(speedtestExpectBodyDrained)
	}
	if !tb.closed {
		t.Fatal(speedtestExpectBodyClosed)
	}
}
