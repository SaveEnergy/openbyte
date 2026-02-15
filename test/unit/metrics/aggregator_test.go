package metrics_test

import (
	"sync"
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

func TestMultiStreamAggregatorConcurrentGetMetrics(t *testing.T) {
	agg := metrics.NewMultiStreamAggregator(4)
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				agg.RecordBytes(128, "sent")
				agg.RecordBytes(128, "recv")
				agg.RecordLatency(5 * time.Millisecond)
				_ = agg.GetAggregatedMetrics()
			}
		}()
	}
	wg.Wait()

	got := agg.GetAggregatedMetrics()
	if got.BytesTransferred == 0 {
		t.Fatalf("expected aggregated bytes > 0")
	}
}
