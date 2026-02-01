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
	handler := api.NewSpeedTestHandler(10)

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
	handler := api.NewSpeedTestHandler(10)

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

func TestSpeedTestUploadHandlesReadError(t *testing.T) {
	handler := api.NewSpeedTestHandler(10)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/upload", nil)
	req.Body = io.NopCloser(&errReader{})
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
