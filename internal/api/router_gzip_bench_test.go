package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkGzipMiddlewareSmallAsset compresses a small static-sized payload when Accept-Encoding: gzip.
func BenchmarkGzipMiddlewareSmallAsset(b *testing.B) {
	payload := bytes.Repeat([]byte("x"), 8192)
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})
	h := gzipMiddleware(inner)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("code=%d", w.Code)
		}
	}
}
