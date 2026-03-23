package jsonbody

import (
	"fmt"
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

// BenchmarkDecodeSingleObjectLarge uses a multi-field JSON body similar in size to results/API saves.
func BenchmarkDecodeSingleObjectLarge(b *testing.B) {
	parts := make([]string, 0, 80)
	for i := range 80 {
		parts = append(parts, fmt.Sprintf(`"k%d":"%016x"`, i, i*0x12345))
	}
	inner := strings.Join(parts, ",")
	body := `{"id":"abc123","meta":{"tags":["x","y"],` + inner + `,"z":true},"payload":{"n":42,"ok":true}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	var dst map[string]any

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req.Body = io.NopCloser(strings.NewReader(body))
		if err := DecodeSingleObject(w, req, &dst, 65536); err != nil {
			b.Fatal(err)
		}
	}
}
