package jsonbody

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type benchDecodePayload struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// BenchmarkDecodeSingleObject measures typical API/results JSON POST bodies.
func BenchmarkDecodeSingleObject(b *testing.B) {
	const body = `{"name":"openbyte","value":42}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	var dst benchDecodePayload
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req.Body = io.NopCloser(strings.NewReader(body))
		if err := DecodeSingleObject(w, req, &dst, 4096); err != nil {
			b.Fatal(err)
		}
	}
}
