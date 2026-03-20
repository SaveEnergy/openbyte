package metrics

import (
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (c *Collector) GetMetrics() types.Metrics {
	bytesSent := atomic.LoadInt64(&c.bytesSent)
	bytesRecv := atomic.LoadInt64(&c.bytesRecv)
	packetsSent := atomic.LoadInt64(&c.packetsSent)
	packetsRecv := atomic.LoadInt64(&c.packetsRecv)

	var latencyMetrics types.LatencyMetrics
	var jitterMs float64
	var (
		startTime    time.Time
		latencyCount int64
		latencyMin   time.Duration
		latencyMax   time.Duration
		latencySum   time.Duration
		jitterSum    time.Duration
		jitterCount  int64
	)
	c.mu.RLock()
	startTime = c.startTime
	latencyCount = c.latencyCount
	latencyMin = c.latencyMin
	latencyMax = c.latencyMax
	latencySum = c.latencySum
	jitterSum = c.jitterSum
	jitterCount = c.jitterCount
	c.mu.RUnlock()
	if c.latencyHistogram != nil && latencyCount > 0 {
		buckets, ok := c.bucketPool.Get().([]uint32)
		if !ok || len(buckets) != c.latencyHistogram.BucketCount() {
			buckets = make([]uint32, c.latencyHistogram.BucketCount())
		}
		overflow := c.latencyHistogram.CopyTo(buckets)
		latencyMetrics = CalculateLatencyFromHistogram(buckets, overflow, c.latencyHistogram.BucketWidth(), latencyCount, latencyMin, latencyMax, latencySum)
		c.bucketPool.Put(buckets)
	}
	if jitterCount > 0 {
		jitterMs = float64(jitterSum) / float64(jitterCount) / float64(time.Millisecond)
	}

	elapsed := time.Since(startTime)
	totalBytes := bytesSent + bytesRecv

	throughputMbps := float64(0)
	throughputAvgMbps := float64(0)
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
		throughputAvgMbps = throughputMbps
	}

	packetLoss := float64(0)
	if packetsSent > 0 && packetsSent > packetsRecv {
		packetLoss = float64(packetsSent-packetsRecv) / float64(packetsSent) * 100
	}

	return types.Metrics{
		ThroughputMbps:     throughputMbps,
		ThroughputAvgMbps:  throughputAvgMbps,
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
