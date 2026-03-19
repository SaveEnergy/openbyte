package client

import (
	"context"
	"fmt"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// SpeedTestOptions configures a full speed test.
type SpeedTestOptions struct {
	Direction string // "download" or "upload" (default: "download")
	Duration  int    // seconds, 1-300 (default: 10)
}

// SpeedTestResult is the output of a full speed test.
type SpeedTestResult struct {
	Direction      string                     `json:"direction"`
	ServerURL      string                     `json:"server_url"`
	ThroughputMbps float64                    `json:"throughput_mbps"`
	LatencyMs      float64                    `json:"latency_ms"`
	JitterMs       float64                    `json:"jitter_ms"`
	BytesTotal     int64                      `json:"bytes_transferred"`
	DurationSec    float64                    `json:"duration_seconds"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

// SpeedTest runs a full speed test with configurable duration and direction.
func (c *Client) SpeedTest(ctx context.Context, opts SpeedTestOptions) (*SpeedTestResult, error) {
	opts, err := normalizeSpeedTestOptions(opts)
	if err != nil {
		return nil, err
	}

	testCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Duration+15)*time.Second)
	defer cancel()

	if err := c.healthCheck(testCtx); err != nil {
		return nil, err
	}

	avgLatency, jitter, latencyOK := c.measureLatency(testCtx, 5)
	if !latencyOK {
		return nil, ErrLatencyMeasurementFailed
	}

	start := time.Now()
	var throughput float64
	var totalBytes int64

	throughput, totalBytes, throughputOK := c.measureThroughput(testCtx, opts)
	if !throughputOK {
		if opts.Direction == directionDownload {
			return nil, ErrDownloadMeasurementFailed
		}
		return nil, ErrUploadMeasurementFailed
	}
	elapsed := time.Since(start)

	downMbps, upMbps := splitThroughputByDirection(opts.Direction, throughput)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
		PacketLoss:   -1,
	})

	return &SpeedTestResult{
		Direction:      opts.Direction,
		ServerURL:      c.serverURL,
		ThroughputMbps: throughput,
		LatencyMs:      avgLatency,
		JitterMs:       jitter,
		BytesTotal:     totalBytes,
		DurationSec:    elapsed.Seconds(),
		Interpretation: interp,
	}, nil
}

func normalizeSpeedTestOptions(opts SpeedTestOptions) (SpeedTestOptions, error) {
	if opts.Direction == "" {
		opts.Direction = directionDownload
	}
	if opts.Duration < 1 {
		opts.Duration = 10
	}
	if opts.Duration > 300 {
		opts.Duration = 300
	}
	if opts.Direction != directionDownload && opts.Direction != directionUpload {
		return SpeedTestOptions{}, fmt.Errorf("invalid direction: %s (must be download or upload)", opts.Direction)
	}
	return opts, nil
}

func (c *Client) measureThroughput(ctx context.Context, opts SpeedTestOptions) (float64, int64, bool) {
	if opts.Direction == directionDownload {
		return c.downloadMeasured(ctx, opts.Duration)
	}
	return c.uploadMeasured(ctx, opts.Duration)
}

func splitThroughputByDirection(direction string, throughput float64) (downMbps, upMbps float64) {
	if direction == directionDownload {
		return throughput, 0
	}
	return 0, throughput
}
