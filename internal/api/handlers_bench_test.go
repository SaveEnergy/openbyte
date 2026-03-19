package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

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
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		respondJSON(w, data, 200)
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
