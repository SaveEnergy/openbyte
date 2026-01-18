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
			New: func() interface{} {
				return make([]uint32, latencyBucketCount)
			},
		},
	}
}

func (c *Collector) RecordBytes(bytes int64, direction string) {
	if direction == "sent" {
		atomic.AddInt64(&c.bytesSent, bytes)
	} else {
		atomic.AddInt64(&c.bytesRecv, bytes)
	}
}

func (c *Collector) RecordLatency(latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latencyHistogram != nil {
		c.latencyHistogram.Record(latency)
	}
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

func (c *Collector) GetMetrics() types.Metrics {
	bytesSent := atomic.LoadInt64(&c.bytesSent)
	bytesRecv := atomic.LoadInt64(&c.bytesRecv)
	packetsSent := atomic.LoadInt64(&c.packetsSent)
	packetsRecv := atomic.LoadInt64(&c.packetsRecv)

	var latencyMetrics types.LatencyMetrics
	var jitterMs float64
	c.mu.RLock()
	if c.latencyHistogram != nil && c.latencyCount > 0 {
		buckets := c.bucketPool.Get().([]uint32)
		if len(buckets) != c.latencyHistogram.BucketCount() {
			buckets = make([]uint32, c.latencyHistogram.BucketCount())
		}
		overflow := c.latencyHistogram.CopyTo(buckets)
		latencyMetrics = CalculateLatencyFromHistogram(buckets, overflow, c.latencyHistogram.BucketWidth(), c.latencyCount, c.latencyMin, c.latencyMax, c.latencySum)
		c.bucketPool.Put(buckets)
	}
	if c.jitterCount > 0 {
		jitterMs = float64(c.jitterSum) / float64(c.jitterCount) / float64(time.Millisecond)
	}
	c.mu.RUnlock()

	elapsed := time.Since(c.startTime)
	totalBytes := bytesSent + bytesRecv

	throughputMbps := float64(0)
	throughputAvgMbps := float64(0)
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
		throughputAvgMbps = throughputMbps
	}

	packetLoss := float64(0)
	if packetsSent > 0 {
		packetLoss = float64(packetsSent-packetsRecv) / float64(packetsSent) * 100
	}

	return types.Metrics{
		ThroughputMbps:    throughputMbps,
		ThroughputAvgMbps: throughputAvgMbps,
		Latency:           latencyMetrics,
		JitterMs:          jitterMs,
		PacketLossPercent: packetLoss,
		BytesTransferred:  totalBytes,
		PacketsSent:       packetsSent,
		PacketsReceived:   packetsRecv,
		Timestamp:         time.Now(),
		StreamCount:       1,
	}
}

func (c *Collector) GetSnapshot() types.Metrics {
	return c.GetMetrics()
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

func (c *Collector) Close() {}

func (c *Collector) SnapshotLatencyStats(dst []uint32) (uint32, int64, time.Duration, time.Duration, time.Duration, time.Duration, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	overflow := uint32(0)
	if c.latencyHistogram != nil {
		overflow = c.latencyHistogram.CopyTo(dst)
	}
	return overflow, c.latencyCount, c.latencyMin, c.latencyMax, c.latencySum, c.jitterSum, c.jitterCount
}
