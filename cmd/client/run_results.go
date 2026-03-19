package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
	"github.com/saveenergy/openbyte/pkg/types"
)

func handleClientTestCompletion(
	ctx context.Context,
	config *Config,
	formatter OutputFormatter,
	streamID string,
	startTime time.Time,
	metrics EngineMetrics,
	runErr error,
) error {
	results := buildResults(streamID, config, metrics, startTime)
	formatter.FormatComplete(results)
	if ferr := formatterLastError(formatter); ferr != nil {
		return fmt.Errorf(formatterOutputFailedFmt, ferr)
	}

	completeErr := completeStream(ctx, config, streamID, metrics)
	if completeErr != nil {
		if shouldReturnRunError(runErr) {
			return fmt.Errorf("%w (and completion report failed: %v)", runErr, completeErr)
		}
		return fmt.Errorf("failed to report completion: %w", completeErr)
	}

	if shouldReturnRunError(runErr) {
		return runErr
	}
	return nil
}

func cancelStreamWithCleanup(ctx context.Context, config *Config, streamID string, rootErr error) error {
	cancelErr := CancelStream(ctx, config.ServerURL, streamID, config.APIKey)
	if cancelErr != nil {
		return fmt.Errorf("%w (and cancel cleanup failed: %v)", rootErr, cancelErr)
	}
	return rootErr
}

func shouldReturnRunError(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)
}

func computePingMetrics(samples []time.Duration) (LatencyStats, float64) {
	if len(samples) == 0 {
		return LatencyStats{}, 0
	}
	return calculateClientLatency(samples), calculateClientJitter(samples)
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
	case directionBidirectional:
		downMbps = throughput / 2
		upMbps = throughput / 2
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
			Protocol:   config.Protocol,
			Direction:  config.Direction,
			Duration:   config.Duration,
			Streams:    config.Streams,
			PacketSize: config.PacketSize,
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
