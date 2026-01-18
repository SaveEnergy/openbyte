package metrics

import (
	"sort"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func CalculateLatency(samples []time.Duration) types.LatencyMetrics {
	if len(samples) == 0 {
		return types.LatencyMetrics{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	min := float64(sorted[0]) / float64(time.Millisecond)
	max := float64(sorted[len(sorted)-1]) / float64(time.Millisecond)

	sum := time.Duration(0)
	for _, s := range sorted {
		sum += s
	}
	avg := float64(sum) / float64(len(sorted)) / float64(time.Millisecond)

	p50 := float64(sorted[len(sorted)*50/100]) / float64(time.Millisecond)
	p95 := float64(sorted[len(sorted)*95/100]) / float64(time.Millisecond)
	p99 := float64(sorted[len(sorted)*99/100]) / float64(time.Millisecond)

	return types.LatencyMetrics{
		MinMs: min,
		MaxMs: max,
		AvgMs: avg,
		P50Ms: p50,
		P95Ms: p95,
		P99Ms: p99,
		Count: len(samples),
	}
}

func CalculateLatencyFromHistogram(bucketCounts []uint32, overflow uint32, bucketWidth time.Duration, count int64, min, max time.Duration, sum time.Duration) types.LatencyMetrics {
	if count == 0 {
		return types.LatencyMetrics{}
	}

	minMs := float64(min) / float64(time.Millisecond)
	maxMs := float64(max) / float64(time.Millisecond)
	avgMs := float64(sum) / float64(count) / float64(time.Millisecond)

	p50 := percentileFromHistogram(bucketCounts, overflow, bucketWidth, count, 0.50, maxMs)
	p95 := percentileFromHistogram(bucketCounts, overflow, bucketWidth, count, 0.95, maxMs)
	p99 := percentileFromHistogram(bucketCounts, overflow, bucketWidth, count, 0.99, maxMs)

	return types.LatencyMetrics{
		MinMs: minMs,
		MaxMs: maxMs,
		AvgMs: avgMs,
		P50Ms: p50,
		P95Ms: p95,
		P99Ms: p99,
		Count: int(count),
	}
}

func percentileFromHistogram(bucketCounts []uint32, overflow uint32, bucketWidth time.Duration, count int64, ratio float64, maxMs float64) float64 {
	if count <= 0 {
		return 0
	}

	target := int64(float64(count)*ratio) + 1
	if target < 1 {
		target = 1
	}

	var seen int64
	for i, c := range bucketCounts {
		seen += int64(c)
		if seen >= target {
			return float64(i+1) * float64(bucketWidth/time.Millisecond)
		}
	}

	if overflow > 0 {
		return maxMs
	}
	if len(bucketCounts) > 0 {
		return float64(len(bucketCounts)) * float64(bucketWidth/time.Millisecond)
	}
	return 0
}

func CalculateJitter(samples []time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	var sum float64
	for i := 1; i < len(samples); i++ {
		diff := float64(samples[i] - samples[i-1])
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}

	return sum / float64(len(samples)-1) / float64(time.Millisecond)
}
