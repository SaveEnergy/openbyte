package metrics

import (
	"slices"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func CalculateLatency(samples []time.Duration) types.LatencyMetrics {
	if len(samples) == 0 {
		return types.LatencyMetrics{}
	}

	sorted := make([]time.Duration, len(samples))
	var sum time.Duration
	for i := range samples {
		sorted[i] = samples[i]
		sum += samples[i]
	}
	slices.Sort(sorted)

	min := float64(sorted[0]) / float64(time.Millisecond)
	max := float64(sorted[len(sorted)-1]) / float64(time.Millisecond)

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

	bucketWidthMs := float64(bucketWidth / time.Millisecond)
	p50 := percentileFromHistogram(bucketCounts, overflow, count, 1, 2, maxMs, bucketWidthMs)
	p95 := percentileFromHistogram(bucketCounts, overflow, count, 95, 100, maxMs, bucketWidthMs)
	p99 := percentileFromHistogram(bucketCounts, overflow, count, 99, 100, maxMs, bucketWidthMs)

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

func percentileFromHistogram(bucketCounts []uint32, overflow uint32, count int64, num, denom int64, maxMs float64, bucketWidthMs float64) float64 {
	if count <= 0 {
		return 0
	}
	if num <= 0 || denom <= 0 {
		return 0
	}

	// Integer ceil(count * num / denom), matching legacy math.Ceil(float64(count)*ratio).
	target := max((count*num+denom-1)/denom, 1)

	var seen int64
	for i, c := range bucketCounts {
		seen += int64(c)
		if seen >= target {
			return float64(i+1) * bucketWidthMs
		}
	}

	if overflow > 0 {
		return maxMs
	}
	if len(bucketCounts) > 0 {
		return float64(len(bucketCounts)) * bucketWidthMs
	}
	return 0
}

func CalculateJitter(samples []time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	var sum float64
	for i := 1; i < len(samples); i++ {
		d := samples[i] - samples[i-1]
		if d < 0 {
			d = -d
		}
		sum += float64(d)
	}

	return sum / float64(len(samples)-1) / float64(time.Millisecond)
}
