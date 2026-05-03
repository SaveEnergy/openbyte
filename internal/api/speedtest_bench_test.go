package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

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

// BenchmarkParseDownloadParams measures query parsing for GET /download (typical UI query string).
func BenchmarkParseDownloadParams(b *testing.B) {
	req := httptest.NewRequest("GET", "/api/v1/download?duration=60&chunk=1048576", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _, err := parseDownloadParams(req, 300)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadUploadBody drains a fixed-size body through the speedtest upload read loop (buffer pool + Read).
func BenchmarkReadUploadBody(b *testing.B) {
	const bodySize = 4 * 1024 * 1024
	data := make([]byte, bodySize)
	pool := &sync.Pool{
		New: func() any {
			return newUploadBuffer()
		},
	}
	ctx := context.Background()
	deadline := time.Now().Add(24 * time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		body := bytes.NewReader(data)
		n, failed := readUploadBody(ctx, body, nil, deadline, pool)
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
