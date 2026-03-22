package jsonbody

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

type benchDecodePayload struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// BenchmarkDecodeSingleObject measures typical API/results JSON POST bodies.
func BenchmarkDecodeSingleObject(b *testing.B) {
	const body = `{"name":"openbyte","value":42}`
	bodyBytes := []byte(body)
	w := httptest.NewRecorder()
	var dst benchDecodePayload
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		if err := DecodeSingleObject(w, req, &dst, 4096); err != nil {
			b.Fatal(err)
		}
	}
}
