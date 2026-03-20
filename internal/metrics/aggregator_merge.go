package metrics

import "time"

type latencyMergeTotals struct {
	overflow          uint32
	totalLatencyCount int64
	totalLatencySum   time.Duration
	totalLatencyMin   time.Duration
	totalLatencyMax   time.Duration
	hasLatency        bool
	totalJitterSum    time.Duration
	totalJitterCount  int64
}

func mergeLatencySnapshot(
	bucketCounts, bucketScratch []uint32,
	snap LatencySnapshot,
	totals *latencyMergeTotals,
) {
	totals.overflow += snap.Overflow
	totals.totalLatencyCount += snap.Count
	totals.totalLatencySum += snap.Sum
	if snap.Count > 0 {
		if !totals.hasLatency || snap.Min < totals.totalLatencyMin {
			totals.totalLatencyMin = snap.Min
		}
		if !totals.hasLatency || snap.Max > totals.totalLatencyMax {
			totals.totalLatencyMax = snap.Max
		}
		totals.hasLatency = true
	}
	totals.totalJitterSum += snap.JitterSum
	totals.totalJitterCount += snap.JitterCount
	for i, v := range bucketScratch {
		bucketCounts[i] += v
	}
}
