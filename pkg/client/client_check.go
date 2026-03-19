package client

import (
	"context"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// CheckResult is the output of a quick connectivity check.
type CheckResult struct {
	Status         string                     `json:"status"`
	ServerURL      string                     `json:"server_url"`
	LatencyMs      float64                    `json:"latency_ms"`
	DownloadMbps   float64                    `json:"download_mbps"`
	UploadMbps     float64                    `json:"upload_mbps"`
	JitterMs       float64                    `json:"jitter_ms"`
	DurationMs     int64                      `json:"duration_ms"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

// Check runs a quick ~3-5 second connectivity check (latency + burst download + burst upload).
func (c *Client) Check(ctx context.Context) (*CheckResult, error) {
	start := time.Now()

	if err := c.healthCheck(ctx); err != nil {
		return nil, err
	}

	avgLatency, jitter, latencyOK := c.measureLatency(ctx, 5)
	downMbps, downOK := c.downloadBurst(ctx, 2)
	upMbps, upOK := c.uploadBurst(ctx, 2)
	if !latencyOK {
		return nil, ErrLatencyMeasurementFailed
	}
	if !downOK {
		return nil, ErrDownloadMeasurementFailed
	}
	if !upOK {
		return nil, ErrUploadMeasurementFailed
	}

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
		PacketLoss:   -1,
	})

	return &CheckResult{
		Status:         "ok",
		ServerURL:      c.serverURL,
		LatencyMs:      avgLatency,
		DownloadMbps:   downMbps,
		UploadMbps:     upMbps,
		JitterMs:       jitter,
		DurationMs:     time.Since(start).Milliseconds(),
		Interpretation: interp,
	}, nil
}
