package api

import (
	"bytes"
	"net/http"
	"testing"
)

// benchJSONWriter is a minimal [http.ResponseWriter] for BenchmarkRespondJSON
// (avoids per-iteration [httptest.NewRecorder] allocations).
type benchJSONWriter struct {
	hdr http.Header
	buf bytes.Buffer
}

func (w *benchJSONWriter) Header() http.Header {
	if w.hdr == nil {
		w.hdr = make(http.Header)
	}
	return w.hdr
}

func (w *benchJSONWriter) WriteHeader(int) {}

func (w *benchJSONWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func BenchmarkRespondJSON(b *testing.B) {
	data := map[string]any{
		"version": "0.0.0+bench",
		"commit":  "deadbeef",
	}
	var w benchJSONWriter
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w.buf.Reset()
		for k := range w.hdr {
			delete(w.hdr, k)
		}
		respondJSON(&w, data, 200)
	}
}
