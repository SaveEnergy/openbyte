package websocket_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type benchMessage struct {
	Type             string        `json:"type"`
	StreamID         string        `json:"stream_id"`
	Status           string        `json:"status"`
	Progress         float64       `json:"progress,omitempty"`
	ElapsedSeconds   float64       `json:"elapsed_seconds,omitempty"`
	RemainingSeconds float64       `json:"remaining_seconds,omitempty"`
	Metrics          types.Metrics `json:"metrics"`
	Time             int64         `json:"time"`
}

func BenchmarkEncodeMetricsMessage(b *testing.B) {
	msg := benchMessage{
		Type:             "metrics",
		StreamID:         "bench",
		Status:           "running",
		Progress:         50.0,
		ElapsedSeconds:   15.0,
		RemainingSeconds: 15.0,
		Metrics: types.Metrics{
			ThroughputMbps:    25000.5,
			ThroughputAvgMbps: 24500.1,
			Latency: types.LatencyMetrics{
				MinMs: 0.1,
				MaxMs: 2.5,
				AvgMs: 0.5,
				P50Ms: 0.4,
				P95Ms: 1.2,
				P99Ms: 2.0,
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
		Time: time.Now().Unix(),
	}

	// Match internal/websocket marshalMessage: Encoder into a grown buffer, no HTML
	// escape, strip trailing newline (BroadcastMetrics does the same before send).
	var buf bytes.Buffer
	buf.Grow(2048)
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	b.ResetTimer()
	for range b.N {
		buf.Reset()
		if err := enc.Encode(msg); err != nil {
			b.Fatal(err)
		}
		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
			buf.Truncate(buf.Len() - 1)
		}
	}
}
