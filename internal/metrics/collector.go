package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type CollectorInterface interface {
	RecordBytes(bytes int64, direction string)
	RecordLatency(latency time.Duration)
	RecordPacket(sent bool)
	GetMetrics() types.Metrics
	Reset()
	Close()
}

type Collector struct {
	bytesSent        int64
	bytesRecv        int64
	packetsSent      int64
	packetsRecv      int64
	latencyHistogram *LatencyHistogram
	latencySum       time.Duration
	latencyCount     int64
	latencyMin       time.Duration
	latencyMax       time.Duration
	jitterSum        time.Duration
	jitterCount      int64
	lastLatency      time.Duration
	hasLastLatency   bool
	mu               sync.RWMutex
	startTime        time.Time
	bucketPool       sync.Pool
}

const (
	latencyBucketWidth = time.Millisecond
	latencyBucketCount = 2000
)

func NewCollector() *Collector {
	return &Collector{
		latencyHistogram: NewLatencyHistogram(latencyBucketWidth, latencyBucketCount),
		startTime:        time.Now(),
		bucketPool: sync.Pool{
			New: func() any {
				return make([]uint32, latencyBucketCount)
			},
		},
	}
}

func (c *Collector) RecordBytes(bytes int64, direction string) {
	if bytes <= 0 {
		return
	}
	if direction == "sent" {
		atomic.AddInt64(&c.bytesSent, bytes)
	} else {
		atomic.AddInt64(&c.bytesRecv, bytes)
	}
}

func (c *Collector) RecordLatency(latency time.Duration) {
	// Histogram has its own mutex — record outside c.mu to reduce contention.
	if c.latencyHistogram != nil {
		c.latencyHistogram.Record(latency)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencySum += latency
	c.latencyCount++
	if !c.hasLastLatency {
		c.latencyMin = latency
		c.latencyMax = latency
		c.lastLatency = latency
		c.hasLastLatency = true
		return
	}
	if latency < c.latencyMin {
		c.latencyMin = latency
	}
	if latency > c.latencyMax {
		c.latencyMax = latency
	}
	diff := latency - c.lastLatency
	if diff < 0 {
		diff = -diff
	}
	c.jitterSum += diff
	c.jitterCount++
	c.lastLatency = latency
}

func (c *Collector) RecordPacket(sent bool) {
	if sent {
		atomic.AddInt64(&c.packetsSent, 1)
	} else {
		atomic.AddInt64(&c.packetsRecv, 1)
	}
}

func (c *Collector) Reset() {
	atomic.StoreInt64(&c.bytesSent, 0)
	atomic.StoreInt64(&c.bytesRecv, 0)
	atomic.StoreInt64(&c.packetsSent, 0)
	atomic.StoreInt64(&c.packetsRecv, 0)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latencyHistogram != nil {
		c.latencyHistogram.Reset()
	}
	c.latencySum = 0
	c.latencyCount = 0
	c.latencyMin = 0
	c.latencyMax = 0
	c.jitterSum = 0
	c.jitterCount = 0
	c.lastLatency = 0
	c.hasLastLatency = false
	c.startTime = time.Now()
}

func (c *Collector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Do not set c.latencyHistogram = nil to avoid data races with RecordLatency
	// which reads it outside the lock to minimize contention.
	c.bucketPool = sync.Pool{}
}

// LatencySnapshot holds a point-in-time copy of latency statistics.
type LatencySnapshot struct {
	Overflow    uint32
	Count       int64
	Min         time.Duration
	Max         time.Duration
	Sum         time.Duration
	JitterSum   time.Duration
	JitterCount int64
}

func (c *Collector) SnapshotLatencyStats(dst []uint32) LatencySnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	overflow := uint32(0)
	if c.latencyHistogram != nil {
		overflow = c.latencyHistogram.CopyTo(dst)
	}
	return LatencySnapshot{
		Overflow:    overflow,
		Count:       c.latencyCount,
		Min:         c.latencyMin,
		Max:         c.latencyMax,
		Sum:         c.latencySum,
		JitterSum:   c.jitterSum,
		JitterCount: c.jitterCount,
	}
}
