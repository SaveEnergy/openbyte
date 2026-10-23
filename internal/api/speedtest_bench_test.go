package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// BenchmarkUploadHandler includes slot accounting, request deadlines and teardown.
func BenchmarkUploadHandler(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{{"64KiB", 64 << 10}, {"8MiB", 8 << 20}} {
		b.Run(size.name, func(b *testing.B) {
			handler := NewSpeedTestHandlerWithPolicy(200, 300, 64, nil)
			data := make([]byte, size.bytes)
			b.SetBytes(int64(size.bytes))
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
				for pb.Next() {
					req.Body = io.NopCloser(bytes.NewReader(data))
					req.ContentLength = int64(len(data))
					w := httptest.NewRecorder()
					handler.Upload(w, req)
					if w.Code != http.StatusOK {
						b.Errorf("upload status = %d", w.Code)
					}
				}
			})
		})
	}
}

// BenchmarkWriteChunkFromSource mirrors HTTP /download chunking (speedtest_download.go) without timers or flushing.
func BenchmarkWriteChunkFromSource(b *testing.B) {
	source := make([]byte, 1024*1024)
	chunkSize := 256 * 1024
	offset := 0
	var w benchJSONWriter

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := writeChunkFromSource(&w, source, chunkSize, &offset); err != nil {
			b.Fatal(err)
		}
		w.buf.Reset()
	}
}

// BenchmarkReadUploadBody drains a fixed-size body through the speedtest upload read loop (buffer pool + Read).
func BenchmarkReadUploadBody(b *testing.B) {
	const bodySize = 4 * 1024 * 1024
	data := make([]byte, bodySize)
	handler := &SpeedTestHandler{
		uploadBufPool: sync.Pool{
			New: func() any { return newUploadBuffer() },
		},
	}
	ctx := context.Background()
	deadline := time.Now().Add(24 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		body := bytes.NewReader(data)
		n, failed := handler.readUploadBody(ctx, body, nil, deadline)
		if failed || n != bodySize {
			b.Fatalf("readUploadBody: n=%d failed=%v", n, failed)
		}
	}
}

// BenchmarkWriteUploadResponse measures JSON encoding + headers for POST /upload completion.
func BenchmarkWriteUploadResponse(b *testing.B) {
	start := time.Now().Add(-2 * time.Second)
	const totalBytes = 8 * 1024 * 1024

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		ctrl := http.NewResponseController(w)
		writeUploadResponse(w, ctrl, totalBytes, start)
	}
}
