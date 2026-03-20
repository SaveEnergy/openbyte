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
	for i := range streamCount {
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

func (m *MultiStreamAggregator) GetAggregatedMetrics() types.Metrics {
	m.mu.RLock()
	collectors := append([]*Collector(nil), m.collectors...)
	startTime := m.startTime
	bucketLen := len(m.bucketCounts)
	bucketScratchLen := len(m.bucketScratch)
	bucketWidth := m.bucketWidth
	m.mu.RUnlock()

	if len(collectors) == 0 {
		return types.Metrics{
			Timestamp:   time.Now(),
			StreamCount: 0,
		}
	}

	var totalBytesSent int64
	var totalBytesRecv int64
	var totalPacketsSent int64
	var totalPacketsRecv int64
	totals := latencyMergeTotals{}
	bucketCounts := make([]uint32, bucketLen)
	bucketScratch := make([]uint32, bucketScratchLen)

	for _, collector := range collectors {
		totalBytesSent += atomic.LoadInt64(&collector.bytesSent)
		totalBytesRecv += atomic.LoadInt64(&collector.bytesRecv)
		totalPacketsSent += atomic.LoadInt64(&collector.packetsSent)
		totalPacketsRecv += atomic.LoadInt64(&collector.packetsRecv)
		if len(bucketScratch) == 0 {
			continue
		}
		for i := range bucketScratch {
			bucketScratch[i] = 0
		}
		snap := collector.SnapshotLatencyStats(bucketScratch)
		mergeLatencySnapshot(bucketCounts, bucketScratch, snap, &totals)
	}

	elapsed := time.Since(startTime)
	totalBytes := totalBytesSent + totalBytesRecv

	throughputMbps := float64(0)
	throughputAvgMbps := float64(0)
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
		throughputAvgMbps = throughputMbps
	}

	latency := types.LatencyMetrics{}
	if totals.totalLatencyCount > 0 && bucketWidth > 0 {
		latency = CalculateLatencyFromHistogram(bucketCounts, totals.overflow, bucketWidth, totals.totalLatencyCount, totals.totalLatencyMin, totals.totalLatencyMax, totals.totalLatencySum)
	}
	jitter := float64(0)
	if totals.totalJitterCount > 0 {
		jitter = float64(totals.totalJitterSum) / float64(totals.totalJitterCount) / float64(time.Millisecond)
	}

	packetLoss := float64(0)
	if totalPacketsSent > 0 && totalPacketsSent > totalPacketsRecv {
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
		StreamCount:       len(collectors),
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
	collectors := m.collectors
	m.mu.RUnlock()
	if len(collectors) == 0 {
		return nil
	}
	idx := atomic.AddUint32(&m.rr, 1)
	n := uint32(len(collectors))
	return collectors[int((idx-1)%n)]
}
