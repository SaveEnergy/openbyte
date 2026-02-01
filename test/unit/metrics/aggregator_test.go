package metrics_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func TestMultiStreamAggregatorRecordsAcrossCollectors(t *testing.T) {
	agg := metrics.NewMultiStreamAggregator(2)

	agg.RecordBytes(100, "sent")
	agg.RecordBytes(100, "sent")
	agg.RecordBytes(50, "recv")
	agg.RecordBytes(50, "recv")
	agg.RecordPacket(true)
	agg.RecordPacket(false)
	agg.RecordLatency(10 * time.Millisecond)
	agg.RecordLatency(20 * time.Millisecond)

	got := agg.GetAggregatedMetrics()
	if got.BytesTransferred != 300 {
		t.Fatalf("bytes transferred = %d, want 300", got.BytesTransferred)
	}
	if got.PacketsSent != 1 || got.PacketsReceived != 1 {
		t.Fatalf("packets sent/recv = %d/%d, want 1/1", got.PacketsSent, got.PacketsReceived)
	}
	if got.Latency.Count != 2 {
		t.Fatalf("latency count = %d, want 2", got.Latency.Count)
	}
}
