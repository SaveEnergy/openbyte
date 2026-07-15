package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"
)

// BenchmarkStaticCachedGzipSmallAsset serves a precompressed static-sized payload.
func BenchmarkStaticCachedGzipSmallAsset(b *testing.B) {
	payload := bytes.Repeat([]byte("x"), 8192)
	webFS := http.FS(fstest.MapFS{
		"openbyte.js": &fstest.MapFile{Data: payload, ModTime: time.Unix(1, 0)},
	})
	h := newStaticAllowlistHandler(webFS)
	warmReq := httptest.NewRequest(http.MethodGet, "/openbyte.js", nil)
	warmReq.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(httptest.NewRecorder(), warmReq)

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
