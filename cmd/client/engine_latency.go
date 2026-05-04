package client

import (
	"slices"
	"time"
)

func calculateClientLatency(samples []time.Duration) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	slices.Sort(sorted)

	var sum time.Duration
	for _, s := range sorted {
		sum += s
	}

	n := len(sorted)
	return LatencyStats{
		MinMs: float64(sorted[0]) / float64(time.Millisecond),
		MaxMs: float64(sorted[n-1]) / float64(time.Millisecond),
		AvgMs: float64(sum) / float64(n) / float64(time.Millisecond),
		P50Ms: float64(sorted[n*50/100]) / float64(time.Millisecond),
		P95Ms: float64(sorted[n*95/100]) / float64(time.Millisecond),
		P99Ms: float64(sorted[n*99/100]) / float64(time.Millisecond),
		Count: n,
	}
}

func calculateClientJitter(samples []time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	var sumDiff float64
	for i := 1; i < len(samples); i++ {
		diff := samples[i] - samples[i-1]
		if diff < 0 {
			diff = -diff
		}
		sumDiff += float64(diff)
	}

	return sumDiff / float64(len(samples)-1) / float64(time.Millisecond)
}
