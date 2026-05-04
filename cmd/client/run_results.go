package client

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
	"github.com/saveenergy/openbyte/pkg/types"
)

func computePingMetrics(samples []time.Duration) (LatencyStats, float64) {
	if len(samples) == 0 {
		return LatencyStats{}, 0
	}
	return calculateClientLatency(samples), calculateClientJitter(samples)
}

func shouldReturnRunError(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)
}

func buildResults(streamID string, config *Config, metrics EngineMetrics, startTime time.Time) *StreamResults {
	endTime := time.Now()

	throughput := metrics.ThroughputMbps
	var downMbps, upMbps float64
	switch config.Direction {
	case directionDownload:
		downMbps = throughput
	case directionUpload:
		upMbps = throughput
	}

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    metrics.Latency.AvgMs,
		JitterMs:     metrics.JitterMs,
		PacketLoss:   0,
	})

	return &StreamResults{
		SchemaVersion: SchemaVersion,
		StreamID:      streamID,
		Status:        statusCompleted,
		Config: &StreamConfig{
			Protocol:  protocolHTTP,
			Direction: config.Direction,
			Duration:  config.Duration,
			Streams:   config.Streams,
			ChunkSize: config.ChunkSize,
		},
		Results: &ResultMetrics{
			ThroughputMbps:    metrics.ThroughputMbps,
			ThroughputAvgMbps: metrics.ThroughputMbps,
			LatencyMs: types.LatencyMetrics{
				MinMs: metrics.Latency.MinMs,
				MaxMs: metrics.Latency.MaxMs,
				AvgMs: metrics.Latency.AvgMs,
				P50Ms: metrics.Latency.P50Ms,
				P95Ms: metrics.Latency.P95Ms,
				P99Ms: metrics.Latency.P99Ms,
				Count: metrics.Latency.Count,
			},
			RTT:               metrics.RTT,
			JitterMs:          metrics.JitterMs,
			PacketLossPercent: 0,
			BytesTransferred:  metrics.BytesTransferred,
			PacketsSent:       0,
			PacketsReceived:   0,
			Network:           metrics.Network,
		},
		Interpretation:  interp,
		StartTime:       startTime.Format(time.RFC3339),
		EndTime:         endTime.Format(time.RFC3339),
		DurationSeconds: endTime.Sub(startTime).Seconds(),
	}
}

func createFormatter(config *Config) OutputFormatter {
	if config.JSON {
		return &JSONFormatter{Writer: os.Stdout}
	}
	if config.NDJSON {
		return &NDJSONFormatter{Writer: os.Stdout}
	}
	if config.Plain {
		return NewPlainFormatter(os.Stdout, config.Verbose, config.NoColor, config.NoProgress)
	}
	return NewInteractiveFormatter(os.Stdout, config.Verbose, config.NoColor, config.NoProgress)
}
