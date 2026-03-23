package metrics

import (
	"testing"
	"time"
)

func BenchmarkCollectorGetMetrics(b *testing.B) {
	collector := NewCollector()
	for i := range 10000 {
		collector.RecordLatency(time.Duration(i%2000) * time.Millisecond)
		collector.RecordBytes(1024, "recv")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = collector.GetMetrics()
	}
}

// BenchmarkCollectorRecordLatency measures the hot path used during stream/WS ticks.
func BenchmarkCollectorRecordLatency(b *testing.B) {
	c := NewCollector()
	d := 5 * time.Millisecond
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		c.RecordLatency(d)
	}
}

// BenchmarkLatencyHistogramRecord isolates fixed-bucket histogram updates (see LatencyHistogram.Record).
func BenchmarkLatencyHistogramRecord(b *testing.B) {
	h := NewLatencyHistogram(latencyBucketWidth, latencyBucketCount)
	d := 5 * time.Millisecond
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.Record(d)
	}
}

// BenchmarkMultiStreamAggregatorGetAggregatedMetrics is the fan-in path used for multi-stream tests (WS/API metrics).
func BenchmarkMultiStreamAggregatorGetAggregatedMetrics(b *testing.B) {
	agg := NewMultiStreamAggregator(4)
	for i := range 2000 {
		agg.RecordLatency(time.Duration(i%500) * time.Millisecond)
		agg.RecordBytes(1024, "recv")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = agg.GetAggregatedMetrics()
	}
}

// BenchmarkCalculateLatencyFromHistogram stresses percentileFromHistogram over a full 1ms × 2000 bucket layout.
func BenchmarkCalculateLatencyFromHistogram(b *testing.B) {
	buckets := make([]uint32, latencyBucketCount)
	var total int64
	var sum time.Duration
	for i := range buckets {
		c := uint32((i * 17) % 4)
		buckets[i] = c
		total += int64(c)
		sum += time.Duration(i) * latencyBucketWidth * time.Duration(c)
	}
	min := time.Duration(0)
	max := time.Duration(latencyBucketCount-1) * latencyBucketWidth

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = CalculateLatencyFromHistogram(buckets, 0, latencyBucketWidth, total, min, max, sum)
	}
}

// BenchmarkMultiStreamAggregatorRecordLatency is the round-robin record path on a multi-stream run.
func BenchmarkMultiStreamAggregatorRecordLatency(b *testing.B) {
	agg := NewMultiStreamAggregator(4)
	d := 3 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		agg.RecordLatency(d)
	}
}

// BenchmarkCollectorRecordBytes is the atomic byte counter path (recv/sent totals).
func BenchmarkCollectorRecordBytes(b *testing.B) {
	c := NewCollector()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		c.RecordBytes(8192, "recv")
	}
}

// BenchmarkMultiStreamAggregatorRecordBytes is round-robin byte accounting across streams.
func BenchmarkMultiStreamAggregatorRecordBytes(b *testing.B) {
	agg := NewMultiStreamAggregator(4)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		agg.RecordBytes(4096, "sent")
	}
}

// BenchmarkCollectorRecordPacket counts sent/recv packets (atomic hot path).
func BenchmarkCollectorRecordPacket(b *testing.B) {
	c := NewCollector()

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		c.RecordPacket(i%2 == 0)
	}
}

// BenchmarkMultiStreamAggregatorRecordPacket round-robins packet counters across streams.
func BenchmarkMultiStreamAggregatorRecordPacket(b *testing.B) {
	agg := NewMultiStreamAggregator(4)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		agg.RecordPacket(i%2 == 0)
	}
}

// BenchmarkCalculateLatencySamples is the sort-based percentile path (legacy / supplemental stats).
func BenchmarkCalculateLatencySamples(b *testing.B) {
	samples := make([]time.Duration, 500)
	for i := range samples {
		samples[i] = time.Duration((i*17)%200) * time.Millisecond
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = CalculateLatency(samples)
	}
}

// BenchmarkCalculateJitterSamples measures consecutive-sample jitter aggregation.
func BenchmarkCalculateJitterSamples(b *testing.B) {
	samples := make([]time.Duration, 200)
	for i := range samples {
		samples[i] = time.Duration(10+(i%5)) * time.Millisecond
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = CalculateJitter(samples)
	}
}
