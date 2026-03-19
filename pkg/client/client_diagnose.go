package client

import (
	"context"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// DiagnoseResult is the output of a comprehensive network diagnosis.
type DiagnoseResult struct {
	ServerURL      string                     `json:"server_url"`
	LatencyMs      float64                    `json:"latency_ms"`
	JitterMs       float64                    `json:"jitter_ms"`
	DownloadMbps   float64                    `json:"download_mbps"`
	UploadMbps     float64                    `json:"upload_mbps"`
	DurationMs     int64                      `json:"duration_ms"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

// Diagnose runs a comprehensive network diagnosis: 10 latency samples,
// 5-second download burst, 5-second upload burst, and full interpretation.
// Takes ~15-20 seconds.
func (c *Client) Diagnose(ctx context.Context) (*DiagnoseResult, error) {
	diagCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	start := time.Now()

	if err := c.healthCheck(diagCtx); err != nil {
		return nil, err
	}

	avgLatency, jitter, latencyOK := c.measureLatency(diagCtx, 10)
	downMbps, downOK := c.downloadBurst(diagCtx, 5)
	upMbps, upOK := c.uploadBurst(diagCtx, 5)
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

	return &DiagnoseResult{
		ServerURL:      c.serverURL,
		LatencyMs:      avgLatency,
		JitterMs:       jitter,
		DownloadMbps:   downMbps,
		UploadMbps:     upMbps,
		DurationMs:     time.Since(start).Milliseconds(),
		Interpretation: interp,
	}, nil
}

// Healthy returns nil if the server is reachable and healthy.
func (c *Client) Healthy(ctx context.Context) error {
	return c.healthCheck(ctx)
}
