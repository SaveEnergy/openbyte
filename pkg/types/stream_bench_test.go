package types

import (
	"testing"
	"time"
)

// BenchmarkStreamStateGetState is the read-copy path used when building metrics updates (manager broadcast).
func BenchmarkStreamStateGetState(b *testing.B) {
	ss := &StreamState{
		Config: StreamConfig{
			Protocol:   ProtocolTCP,
			Direction:  DirectionDownload,
			Duration:   60 * time.Second,
			Streams:    4,
			PacketSize: 1400,
		},
		Status:    StreamStatusRunning,
		Progress:  33.3,
		StartTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Metrics: Metrics{
			ThroughputMbps: 500,
			Latency:        LatencyMetrics{Count: 10, AvgMs: 2.5},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = ss.GetState()
	}
}

// BenchmarkStreamStateUpdateMetrics is the write path on each metrics tick (progress + metrics copy).
func BenchmarkStreamStateUpdateMetrics(b *testing.B) {
	ss := &StreamState{
		Config: StreamConfig{
			Duration: 60 * time.Second,
		},
		Status:    StreamStatusRunning,
		StartTime: time.Now().Add(-30 * time.Second),
	}
	m := Metrics{
		ThroughputMbps:    900.1,
		ThroughputAvgMbps: 880.0,
		Latency:           LatencyMetrics{Count: 100, P50Ms: 1.2, P95Ms: 4.0},
		BytesTransferred:  1 << 20,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ss.UpdateMetrics(m)
	}
}
