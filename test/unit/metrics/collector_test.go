package metrics_test

import (
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func TestCollectorRecordBytes(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordBytes(1000, "sent")
	c.RecordBytes(2000, "sent")

	c.RecordBytes(500, "recv")
	c.RecordBytes(1500, "recv")

	metrics := c.GetMetrics()
	if metrics.BytesTransferred != 5000 {
		t.Errorf("BytesTransferred = %d, want 5000", metrics.BytesTransferred)
	}
}

func TestCollectorRecordBytesIgnoresNegative(t *testing.T) {
	c := metrics.NewCollector()
	c.RecordBytes(100, "sent")
	c.RecordBytes(-50, "sent")
	c.RecordBytes(-25, "recv")

	m := c.GetMetrics()
	if m.BytesTransferred != 100 {
		t.Fatalf("BytesTransferred = %d, want 100", m.BytesTransferred)
	}
}

func TestCollectorRecordPacket(t *testing.T) {
	c := metrics.NewCollector()

	for range 10 {
		c.RecordPacket(true)
		c.RecordPacket(false)
	}

	metrics := c.GetMetrics()
	if metrics.PacketsSent != 10 {
		t.Errorf("PacketsSent = %d, want 10", metrics.PacketsSent)
	}
	if metrics.PacketsReceived != 10 {
		t.Errorf("PacketsReceived = %d, want 10", metrics.PacketsReceived)
	}
}

func TestCollectorRecordLatency(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordLatency(10 * time.Millisecond)
	c.RecordLatency(20 * time.Millisecond)
	c.RecordLatency(30 * time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.Latency.Count != 3 {
		t.Errorf("Latency.Count = %d, want 3", metrics.Latency.Count)
	}
}

func TestCollectorConcurrent(t *testing.T) {
	c := metrics.NewCollector()
	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			c.RecordBytes(100, "sent")
			c.RecordPacket(true)
			c.RecordLatency(10 * time.Millisecond)
		})
	}

	wg.Wait()

	metrics := c.GetMetrics()
	if metrics.BytesTransferred != 10000 {
		t.Errorf("BytesTransferred = %d, want 10000", metrics.BytesTransferred)
	}
	if metrics.PacketsSent != 100 {
		t.Errorf("PacketsSent = %d, want 100", metrics.PacketsSent)
	}
}

func TestCollectorConcurrentRecordAndRead(t *testing.T) {
	c := metrics.NewCollector()
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writers: record latency concurrently
	for range 10 {
		wg.Go(func() {
			for j := range 1000 {
				select {
				case <-done:
					return
				default:
					c.RecordLatency(time.Duration(j%100) * time.Millisecond)
				}
			}
		})
	}

	// Readers: get metrics concurrently with writes
	for range 5 {
		wg.Go(func() {
			for range 500 {
				select {
				case <-done:
					return
				default:
					m := c.GetMetrics()
					_ = m.Latency.Count
				}
			}
		})
	}

	wg.Wait()
	close(done)

	m := c.GetMetrics()
	if m.Latency.Count == 0 {
		t.Error("expected latency samples after concurrent record+read")
	}
}

func TestCollectorReset(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordBytes(1000, "sent")
	c.RecordPacket(true)
	c.RecordLatency(10 * time.Millisecond)

	c.Reset()

	metrics := c.GetMetrics()
	if metrics.BytesTransferred != 0 {
		t.Errorf("BytesTransferred = %d, want 0 after reset", metrics.BytesTransferred)
	}
	if metrics.PacketsSent != 0 {
		t.Errorf("PacketsSent = %d, want 0 after reset", metrics.PacketsSent)
	}
	if metrics.Latency.Count != 0 {
		t.Errorf("Latency.Count = %d, want 0 after reset", metrics.Latency.Count)
	}
}
