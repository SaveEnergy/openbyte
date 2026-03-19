package client

import (
	"fmt"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

const formatterOutputFailedFmt = "formatter output failed: %w"

func computeProgress(elapsed time.Duration, totalSeconds float64) (progress, remaining float64) {
	progress = (elapsed.Seconds() / totalSeconds) * 100
	if progress > 100 {
		progress = 100
	}
	remaining = totalSeconds - elapsed.Seconds()
	if remaining < 0 {
		remaining = 0
	}
	return progress, remaining
}

func emitProgressAndMetrics(
	formatter OutputFormatter,
	metrics *types.Metrics,
	progress float64,
	elapsedSeconds float64,
	remaining float64,
) error {
	formatter.FormatProgress(progress, elapsedSeconds, remaining)
	formatter.FormatMetrics(metrics)
	if ferr := formatterLastError(formatter); ferr != nil {
		return fmt.Errorf(formatterOutputFailedFmt, ferr)
	}
	return nil
}

func emitCurrentProgress(
	formatter OutputFormatter,
	startTime time.Time,
	totalSeconds float64,
	metrics EngineMetrics,
) error {
	elapsed := time.Since(startTime)
	progress, remaining := computeProgress(elapsed, totalSeconds)
	return emitProgressAndMetrics(
		formatter,
		engineMetricsToTypesMetrics(metrics),
		progress,
		elapsed.Seconds(),
		remaining,
	)
}

func engineMetricsToTypesMetrics(metrics EngineMetrics) *types.Metrics {
	return &types.Metrics{
		ThroughputMbps:    metrics.ThroughputMbps,
		ThroughputAvgMbps: metrics.ThroughputMbps,
		BytesTransferred:  metrics.BytesTransferred,
		Latency: types.LatencyMetrics{
			MinMs: metrics.Latency.MinMs,
			MaxMs: metrics.Latency.MaxMs,
			AvgMs: metrics.Latency.AvgMs,
			P50Ms: metrics.Latency.P50Ms,
			P95Ms: metrics.Latency.P95Ms,
			P99Ms: metrics.Latency.P99Ms,
			Count: metrics.Latency.Count,
		},
		JitterMs:  metrics.JitterMs,
		Timestamp: time.Now(),
	}
}

func formatterLastError(formatter OutputFormatter) error {
	type lastErrorer interface {
		LastError() error
	}
	if fe, ok := formatter.(lastErrorer); ok {
		return fe.LastError()
	}
	return nil
}
