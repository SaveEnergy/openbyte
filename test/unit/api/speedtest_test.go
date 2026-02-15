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

	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/download?duration=1&chunk=65536", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("content-type = %q, want %q", got, "application/octet-stream")
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Fatalf("cache-control header missing")
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-zero download body")
	}
}

func TestSpeedTestUploadReportsBytes(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	payload := bytes.Repeat([]byte("a"), 256*1024)
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Bytes int64 `json:"bytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
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

func TestDownloadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all slots with long-running downloads using signal writers
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)
	started := make([]chan struct{}, maxConcurrent)

	for i := 0; i < maxConcurrent; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})

		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet,
				"http://example.com/api/v1/download?duration=60&chunk=65536", nil)
			req = req.WithContext(ctx)
			sw := &signalWriter{ResponseRecorder: httptest.NewRecorder(), started: ch}
			handler.Download(sw, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for download goroutine to start")
		}
	}

	// New download should get 503
	reqOver := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?duration=1&chunk=65536", nil)
	recOver := httptest.NewRecorder()
	handler.Download(recOver, reqOver)
	if recOver.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when at limit, got %d", recOver.Code)
	}

	// Cancel all running downloads (simulates user pressing cancel)
	for _, cancel := range cancels {
		cancel()
	}
	for i := 0; i < maxConcurrent; i++ {
		<-done
	}

	// After cancellation, new download should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)
	reqAfter := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?duration=1&chunk=65536", nil)
	reqAfter = reqAfter.WithContext(ctx)
	recAfter := httptest.NewRecorder()
	handler.Download(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf("expected 200 after cancel freed slots, got %d", recAfter.Code)
	}
}

func TestDownloadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/download?duration=1&chunk=65536", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if tb.reads == 0 {
		t.Fatalf("expected request body to be drained before returning 503")
	}
	if !tb.closed {
		t.Fatalf("expected request body to be closed")
	}
}

func TestDownloadValidationRejectsDrainBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	tests := []string{
		"http://example.com/api/v1/download?duration=0",
		"http://example.com/api/v1/download?chunk=bad",
	}
	for _, u := range tests {
		tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
		req := httptest.NewRequest(http.MethodGet, u, nil)
		req.Body = tb
		rec := httptest.NewRecorder()

		handler.Download(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("url=%s status = %d, want %d", u, rec.Code, http.StatusBadRequest)
		}
		if tb.reads == 0 {
			t.Fatalf("url=%s expected body to be drained", u)
		}
		if !tb.closed {
			t.Fatalf("url=%s expected body to be closed", u)
		}
	}
}

func TestSpeedTestUploadHandlesReadError(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", nil)
	req.Body = io.NopCloser(&errReader{})
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestSpeedTestUploadReadErrorDrainsBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &failingTrackingBody{}
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
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

	for i := 0; i < maxConcurrent; i++ {
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
			req := httptest.NewRequest(http.MethodPost,
				"http://example.com/api/v1/upload", body)
			rec := httptest.NewRecorder()
			handler.Upload(rec, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for upload goroutine to start")
		}
	}

	// New upload should get 503
	reqOver := httptest.NewRequest(http.MethodPost,
		"http://example.com/api/v1/upload",
		bytes.NewReader([]byte("data")))
	recOver := httptest.NewRecorder()
	handler.Upload(recOver, reqOver)
	if recOver.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when at limit, got %d", recOver.Code)
	}

	// Cancel all running uploads
	for _, cancel := range cancels {
		cancel()
	}
	for i := 0; i < maxConcurrent; i++ {
		<-done
	}

	// After cancellation, new upload should succeed
	reqAfter := httptest.NewRequest(http.MethodPost,
		"http://example.com/api/v1/upload",
		bytes.NewReader(bytes.Repeat([]byte("x"), 1024)))
	recAfter := httptest.NewRecorder()
	handler.Upload(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf("expected 200 after cancel freed slots, got %d", recAfter.Code)
	}
}

func TestUploadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if tb.reads == 0 {
		t.Fatalf("expected request body to be drained before returning 503")
	}
	if !tb.closed {
		t.Fatalf("expected request body to be closed")
	}
}

func TestDownloadRespectsMaxDuration(t *testing.T) {
	// maxDurationSec=5: duration=5 should work, duration=10 should be clamped to default
	handler := api.NewSpeedTestHandler(10, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// duration=5 (within max) should be accepted
	req := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?duration=5&chunk=65536", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.Download(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duration=5 with max=5: status = %d, want 200", rec.Code)
	}

	// duration=10 (above max=5) should be rejected with 400
	req2 := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?duration=10&chunk=65536", nil)
	rec2 := httptest.NewRecorder()
	handler.Download(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("duration=10 with max=5: status = %d, want 400", rec2.Code)
	}

	// invalid chunk should be rejected with 400
	req3 := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?chunk=abc", nil)
	rec3 := httptest.NewRecorder()
	handler.Download(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("chunk=abc: status = %d, want 400", rec3.Code)
	}
}

func TestSpeedTestHandlerPingResponseShape(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/ping", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want %q", got, "application/json")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want %q", got, "no-store")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if pong, ok := resp["pong"].(bool); !ok || !pong {
		t.Fatalf("pong = %v, want true", resp["pong"])
	}
	if _, ok := resp["timestamp"].(float64); !ok {
		t.Fatalf("timestamp missing or wrong type: %T", resp["timestamp"])
	}
	if ip, ok := resp["client_ip"].(string); !ok || ip != "203.0.113.10" {
		t.Fatalf("client_ip = %v, want 203.0.113.10", resp["client_ip"])
	}
	if ipv6, ok := resp["ipv6"].(bool); !ok || ipv6 {
		t.Fatalf("ipv6 = %v, want false", resp["ipv6"])
	}
}

func TestSpeedTestHandlerPingNilResolverFallback(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/ping", nil)
	req.RemoteAddr = "[2001:db8::1]:4242"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ip, ok := resp["client_ip"].(string); !ok || ip != "2001:db8::1" {
		t.Fatalf("client_ip = %v, want 2001:db8::1", resp["client_ip"])
	}
	if ipv6, ok := resp["ipv6"].(bool); !ok || !ipv6 {
		t.Fatalf("ipv6 = %v, want true", resp["ipv6"])
	}
}

func TestSpeedTestHandlerPingDrainsUnexpectedBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
	req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/ping", nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if tb.reads == 0 {
		t.Fatal("expected body to be drained")
	}
	if !tb.closed {
		t.Fatal("expected body to be closed")
	}
}
