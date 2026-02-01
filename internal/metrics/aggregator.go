package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type MultiStreamAggregator struct {
	collectors    []*Collector
	mu            sync.RWMutex
	startTime     time.Time
	bucketCounts  []uint32
	bucketScratch []uint32
	bucketWidth   time.Duration
	rr            uint32
}

var _ CollectorInterface = (*MultiStreamAggregator)(nil)

func NewMultiStreamAggregator(streamCount int) *MultiStreamAggregator {
	collectors := make([]*Collector, streamCount)
	for i := 0; i < streamCount; i++ {
		collectors[i] = NewCollector()
	}

	aggregator := &MultiStreamAggregator{
		collectors: collectors,
		startTime:  time.Now(),
	}
	if len(collectors) > 0 && collectors[0].latencyHistogram != nil {
		aggregator.bucketWidth = collectors[0].latencyHistogram.BucketWidth()
		bucketCount := collectors[0].latencyHistogram.BucketCount()
		aggregator.bucketCounts = make([]uint32, bucketCount)
		aggregator.bucketScratch = make([]uint32, bucketCount)
	}

	return aggregator
}

func (m *MultiStreamAggregator) GetCollector(streamID int) *Collector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if streamID < 0 || streamID >= len(m.collectors) {
		return nil
	}
	return m.collectors[streamID]
}

func (m *MultiStreamAggregator) GetAggregatedMetrics() types.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.collectors) == 0 {
		return types.Metrics{
			Timestamp:   time.Now(),
			StreamCount: 0,
		}
	}

	var totalBytesSent int64
	var totalBytesRecv int64
	var totalPacketsSent int64
	var totalPacketsRecv int64
	var totalLatencyCount int64
	var totalLatencySum time.Duration
	var totalLatencyMin time.Duration
	var totalLatencyMax time.Duration
	hasLatency := false
	var totalJitterSum time.Duration
	var totalJitterCount int64
	var overflow uint32

	for i := range m.bucketCounts {
		m.bucketCounts[i] = 0
	}

	for _, collector := range m.collectors {
		totalBytesSent += atomic.LoadInt64(&collector.bytesSent)
		totalBytesRecv += atomic.LoadInt64(&collector.bytesRecv)
		totalPacketsSent += atomic.LoadInt64(&collector.packetsSent)
		totalPacketsRecv += atomic.LoadInt64(&collector.packetsRecv)
		if len(m.bucketScratch) > 0 {
			for i := range m.bucketScratch {
				m.bucketScratch[i] = 0
			}
			collectorOverflow, count, min, max, sum, jitterSum, jitterCount := collector.SnapshotLatencyStats(m.bucketScratch)
			overflow += collectorOverflow
			totalLatencyCount += count
			totalLatencySum += sum
			if count > 0 {
				if !hasLatency || min < totalLatencyMin {
					totalLatencyMin = min
				}
				if !hasLatency || max > totalLatencyMax {
					totalLatencyMax = max
				}
				hasLatency = true
			}
			totalJitterSum += jitterSum
			totalJitterCount += jitterCount
			for i, v := range m.bucketScratch {
				m.bucketCounts[i] += v
			}
		}
	}

	elapsed := time.Since(m.startTime)
	totalBytes := totalBytesSent + totalBytesRecv

	throughputMbps := float64(0)
	throughputAvgMbps := float64(0)
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
		throughputAvgMbps = throughputMbps
	}

	latency := types.LatencyMetrics{}
	if totalLatencyCount > 0 && m.bucketWidth > 0 {
		latency = CalculateLatencyFromHistogram(m.bucketCounts, overflow, m.bucketWidth, totalLatencyCount, totalLatencyMin, totalLatencyMax, totalLatencySum)
	}
	jitter := float64(0)
	if totalJitterCount > 0 {
		jitter = float64(totalJitterSum) / float64(totalJitterCount) / float64(time.Millisecond)
	}

	packetLoss := float64(0)
	if totalPacketsSent > 0 {
		packetLoss = float64(totalPacketsSent-totalPacketsRecv) / float64(totalPacketsSent) * 100
	}

	return types.Metrics{
		ThroughputMbps:    throughputMbps,
		ThroughputAvgMbps: throughputAvgMbps,
		Latency:           latency,
		JitterMs:          jitter,
		PacketLossPercent: packetLoss,
		BytesTransferred:  totalBytes,
		PacketsSent:       totalPacketsSent,
		PacketsReceived:   totalPacketsRecv,
		Timestamp:         time.Now(),
		StreamCount:       len(m.collectors),
	}
}

func (m *MultiStreamAggregator) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, collector := range m.collectors {
		collector.Reset()
	}
	m.startTime = time.Now()
}

func (m *MultiStreamAggregator) RecordBytes(bytes int64, direction string) {
	collector := m.nextCollector()
	if collector == nil {
		return
	}
	collector.RecordBytes(bytes, direction)
}

func (m *MultiStreamAggregator) RecordLatency(latency time.Duration) {
	collector := m.nextCollector()
	if collector == nil {
		return
	}
	collector.RecordLatency(latency)
}

func (m *MultiStreamAggregator) RecordPacket(sent bool) {
	collector := m.nextCollector()
	if collector == nil {
		return
	}
	collector.RecordPacket(sent)
}

func (m *MultiStreamAggregator) GetMetrics() types.Metrics {
	return m.GetAggregatedMetrics()
}

func (m *MultiStreamAggregator) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, collector := range m.collectors {
		collector.Close()
	}
	m.collectors = nil
}

func (m *MultiStreamAggregator) nextCollector() *Collector {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.collectors) == 0 {
		return nil
	}
	idx := atomic.AddUint32(&m.rr, 1)
	return m.collectors[int(idx-1)%len(m.collectors)]
}
