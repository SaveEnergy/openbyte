package metrics_test

import (
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func TestCollector_RecordBytes(t *testing.T) {
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

func TestCollector_RecordPacket(t *testing.T) {
	c := metrics.NewCollector()

	for i := 0; i < 10; i++ {
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

func TestCollector_RecordLatency(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordLatency(10 * time.Millisecond)
	c.RecordLatency(20 * time.Millisecond)
	c.RecordLatency(30 * time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.Latency.Count != 3 {
		t.Errorf("Latency.Count = %d, want 3", metrics.Latency.Count)
	}
}

func TestCollector_Concurrent(t *testing.T) {
	c := metrics.NewCollector()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.RecordBytes(100, "sent")
			c.RecordPacket(true)
			c.RecordLatency(10 * time.Millisecond)
		}()
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

func TestCollector_ConcurrentRecordAndRead(t *testing.T) {
	c := metrics.NewCollector()
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writers: record latency concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				select {
				case <-done:
					return
				default:
					c.RecordLatency(time.Duration(j%100) * time.Millisecond)
				}
			}
		}()
	}

	// Readers: get metrics concurrently with writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				select {
				case <-done:
					return
				default:
					m := c.GetMetrics()
					_ = m.Latency.Count
				}
			}
		}()
	}

	wg.Wait()
	close(done)

	m := c.GetMetrics()
	if m.Latency.Count == 0 {
		t.Error("expected latency samples after concurrent record+read")
	}
}

func TestCollector_Reset(t *testing.T) {
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
