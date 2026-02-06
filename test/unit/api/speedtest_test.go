package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

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

func TestDownloadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all slots with long-running downloads
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)

	for i := 0; i < maxConcurrent; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel

		go func() {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet,
				"http://example.com/api/v1/download?duration=60&chunk=65536", nil)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handler.Download(rec, req)
		}()
	}

	// Give goroutines time to start and increment activeDownloads
	time.Sleep(50 * time.Millisecond)

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

func TestUploadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all upload slots with long-running uploads
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)

	for i := 0; i < maxConcurrent; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel

		go func() {
			defer func() { done <- struct{}{} }()
			// Slow reader that blocks until context is cancelled
			pr, pw := io.Pipe()
			go func() {
				<-ctx.Done()
				pw.Close()
			}()
			req := httptest.NewRequest(http.MethodPost,
				"http://example.com/api/v1/upload", pr)
			rec := httptest.NewRecorder()
			handler.Upload(rec, req)
		}()
	}

	time.Sleep(50 * time.Millisecond)

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

	// duration=10 (above max) should be silently clamped to default (10s)
	// — since default 10 > max 5, it also gets clamped; verify no crash
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel2)
	req2 := httptest.NewRequest(http.MethodGet,
		"http://example.com/api/v1/download?duration=10&chunk=65536", nil)
	req2 = req2.WithContext(ctx2)
	rec2 := httptest.NewRecorder()
	handler.Download(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("duration=10 with max=5 (clamped): status = %d, want 200", rec2.Code)
	}
}
