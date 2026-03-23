package types

import (
	"encoding/json"
	"testing"
	"time"
)

// BenchmarkMarshalMetricsJSON encodes a full metrics object (API / WS JSON shape).
func BenchmarkMarshalMetricsJSON(b *testing.B) {
	m := Metrics{
		ThroughputMbps:    950.2,
		ThroughputAvgMbps: 920.0,
		Latency: LatencyMetrics{
			MinMs: 0.5, MaxMs: 8.0, AvgMs: 2.1,
			P50Ms: 1.8, P95Ms: 5.0, P99Ms: 7.0,
			Count: 5000,
		},
		RTT: RTTMetrics{
			BaselineMs: 12, CurrentMs: 13, MinMs: 10, MaxMs: 20,
			AvgMs: 12.5, JitterMs: 0.3, Samples: 800,
		},
		JitterMs:          0.4,
		PacketLossPercent: 0.02,
		BytesTransferred:  16 * 1024 * 1024,
		PacketsSent:       12000,
		PacketsReceived:   11950,
		Timestamp:         time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		StreamCount:       4,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := json.Marshal(m)
		if err != nil {
			b.Fatal(err)
		}
	}
}
