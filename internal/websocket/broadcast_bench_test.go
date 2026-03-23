// Microbenches: JSON marshal for BroadcastMetrics. Multi-client fan-out uses live conns in test/unit/websocket.
package websocket

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func benchStreamSnapshot() types.StreamSnapshot {
	start := time.Now().Add(-30 * time.Second)
	return types.StreamSnapshot{
		Config: types.StreamConfig{
			Protocol:   types.ProtocolTCP,
			Direction:  types.DirectionDownload,
			Duration:   60 * time.Second,
			Streams:    4,
			PacketSize: 1400,
		},
		Status:   types.StreamStatusRunning,
		Progress: 50,
		Metrics: types.Metrics{
			ThroughputMbps:    2500.5,
			ThroughputAvgMbps: 2400.1,
			Latency: types.LatencyMetrics{
				MinMs: 0.1, MaxMs: 2.5, AvgMs: 0.5,
				P50Ms: 0.4, P95Ms: 1.2, P99Ms: 2.0,
				Count: 1000,
			},
			JitterMs:          0.15,
			PacketLossPercent: 0.01,
			BytesTransferred:  1024 * 1024,
			PacketsSent:       1000,
			PacketsReceived:   999,
			Timestamp:         time.Now(),
			StreamCount:       4,
		},
		StartTime: start,
	}
}

// BenchmarkMarshalWebsocketMessage uses production marshalMessage + pooled encoder (BroadcastMetrics hot path).
func BenchmarkMarshalWebsocketMessage(b *testing.B) {
	s := NewServer()
	defer s.Close()

	msg := buildWebsocketMessage("bench-stream", benchStreamSnapshot(), wsTypeMetrics)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		st, err := s.marshalMessage(msg)
		if err != nil {
			b.Fatal(err)
		}
		s.wsMarshalPool.Put(st)
	}
}
