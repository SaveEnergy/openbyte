package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

const benchStartStreamBody = `{"protocol":"tcp","direction":"download","duration":60,"streams":4,"packet_size":1400,"mode":"proxy"}`

// BenchmarkDecodeAndValidateStartRequest is JSON decode + defaults + mode validation for POST /stream/start.
func BenchmarkDecodeAndValidateStartRequest(b *testing.B) {
	body := []byte(benchStartStreamBody)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/stream/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(body))
		w := httptest.NewRecorder()
		_, _, ok := decodeAndValidateStartRequest(w, req)
		if !ok {
			b.Fatalf("decode failed: code=%d body=%s", w.Code, w.Body.String())
		}
	}
}

// BenchmarkValidateStreamConfig is config shaping after JSON (validateConfig hot path).
func BenchmarkValidateStreamConfig(b *testing.B) {
	cfg := config.DefaultConfig()
	h := &Handler{config: cfg}
	req := StartStreamRequest{
		Protocol:   "tcp",
		Direction:  "download",
		Duration:   60,
		Streams:    4,
		PacketSize: 1400,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := h.validateConfig(req, "192.0.2.10")
		if err != nil {
			b.Fatal(err)
		}
	}
}
