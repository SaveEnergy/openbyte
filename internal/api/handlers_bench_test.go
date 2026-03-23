package api

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
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

func benchSampleMetrics() types.Metrics {
	return types.Metrics{
		ThroughputMbps:    2500.5,
		ThroughputAvgMbps: 2400.1,
		Latency: types.LatencyMetrics{
			MinMs: 0.1, MaxMs: 2.5, AvgMs: 0.5,
			P50Ms: 0.4, P95Ms: 1.2, P99Ms: 2.0,
			Count: 1000,
		},
		RTT: types.RTTMetrics{
			BaselineMs: 10, CurrentMs: 11, MinMs: 9, MaxMs: 15,
			AvgMs: 10.5, JitterMs: 0.2, Samples: 500,
		},
		JitterMs:          0.15,
		PacketLossPercent: 0.01,
		BytesTransferred:  1024 * 1024,
		PacketsSent:       1000,
		PacketsReceived:   999,
		Timestamp:         time.Unix(1700000000, 0).UTC(),
		StreamCount:       4,
	}
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

func BenchmarkValidateMetricsPayload(b *testing.B) {
	m := benchSampleMetrics()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := validateMetricsPayload(m); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNormalizeHost measures one pass over a fixed set of representative hosts per iteration.
func BenchmarkNormalizeHost(b *testing.B) {
	hosts := []string{
		"",
		"localhost",
		"192.168.1.10:9090",
		"[::1]:443",
		"example.com:8080",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, h := range hosts {
			_ = normalizeHost(h)
		}
	}
}

// BenchmarkIsUnspecifiedBind measures one pass over typical bind addresses per iteration.
func BenchmarkIsUnspecifiedBind(b *testing.B) {
	addrs := []string{
		"",
		"0.0.0.0",
		"0.0.0.0:8080",
		"[::]:8080",
		"[::1]:443",
		"127.0.0.1:9090",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, a := range addrs {
			_ = isUnspecifiedBind(a)
		}
	}
}
