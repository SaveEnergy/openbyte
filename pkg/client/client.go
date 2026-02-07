// Package client provides a Go SDK for running openByte speed tests
// programmatically. Agents and applications can import this package instead
// of shelling out to the CLI.
//
// Usage:
//
//	c := client.New("https://speedtest.example.com")
//	result, err := c.Check(ctx)
//	result, err := c.SpeedTest(ctx, client.SpeedTestOptions{...})
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// Client is an openByte speed test client targeting a single server.
type Client struct {
	serverURL  string
	httpClient *http.Client
	apiKey     string
}

// Option configures the Client.
type Option func(*Client)

// WithAPIKey sets the API key for authenticated requests.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithHTTPClient overrides the default http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a new openByte client targeting the given server URL.
func New(serverURL string, opts ...Option) *Client {
	c := &Client{
		serverURL:  strings.TrimRight(serverURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

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
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()

	if err := c.healthCheck(checkCtx); err != nil {
		return nil, err
	}

	avgLatency, jitter := c.measureLatency(checkCtx, 5)
	downMbps := c.downloadBurst(checkCtx, 2)
	upMbps := c.uploadBurst(checkCtx, 2)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
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
	if opts.Direction == "" {
		opts.Direction = "download"
	}
	if opts.Duration < 1 {
		opts.Duration = 10
	}
	if opts.Duration > 300 {
		opts.Duration = 300
	}
	if opts.Direction != "download" && opts.Direction != "upload" {
		return nil, fmt.Errorf("invalid direction: %s (must be download or upload)", opts.Direction)
	}

	testCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Duration+15)*time.Second)
	defer cancel()

	if err := c.healthCheck(testCtx); err != nil {
		return nil, err
	}

	avgLatency, jitter := c.measureLatency(testCtx, 5)

	start := time.Now()
	var throughput float64
	var totalBytes int64

	if opts.Direction == "download" {
		throughput, totalBytes = c.downloadMeasured(testCtx, opts.Duration)
	} else {
		throughput, totalBytes = c.uploadMeasured(testCtx, opts.Duration)
	}
	elapsed := time.Since(start)

	var downMbps, upMbps float64
	if opts.Direction == "download" {
		downMbps = throughput
	} else {
		upMbps = throughput
	}

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
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

	avgLatency, jitter := c.measureLatency(diagCtx, 10)
	downMbps := c.downloadBurst(diagCtx, 5)
	upMbps := c.uploadBurst(diagCtx, 5)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
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

// --- Internal helpers ---

func (c *Client) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) measureLatency(ctx context.Context, samples int) (avgMs, jitterMs float64) {
	pingURL := c.serverURL + "/api/v1/ping"
	var latencies []time.Duration

	for i := 0; i < samples; i++ {
		if ctx.Err() != nil {
			break
		}
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			continue
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		latencies = append(latencies, time.Since(start))
	}

	if len(latencies) == 0 {
		return 0, 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	avgMs = float64(total) / float64(len(latencies)) / float64(time.Millisecond)

	if len(latencies) >= 2 {
		var jitterSum float64
		for i := 1; i < len(latencies); i++ {
			diff := latencies[i] - latencies[i-1]
			if diff < 0 {
				diff = -diff
			}
			jitterSum += float64(diff) / float64(time.Millisecond)
		}
		jitterMs = jitterSum / float64(len(latencies)-1)
	}

	return avgMs, jitterMs
}

func (c *Client) downloadBurst(ctx context.Context, durationSec int) float64 {
	mbps, _ := c.downloadMeasured(ctx, durationSec)
	return mbps
}

func (c *Client) downloadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64) {
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s/api/v1/download?duration=%d&chunk=1048576", c.serverURL, durationSec)
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0
	}
	req.Header.Set("Accept-Encoding", "identity")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return 0, 0
	}

	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		totalBytes += int64(n)
		if readErr != nil {
			break
		}
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0, totalBytes
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes
}

func (c *Client) uploadBurst(ctx context.Context, durationSec int) float64 {
	mbps, _ := c.uploadMeasured(ctx, durationSec)
	return mbps
}

func (c *Client) uploadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64) {
	upCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()

	payload := make([]byte, 1024*1024)
	totalBytes = int64(len(payload))
	req, err := http.NewRequestWithContext(upCtx, http.MethodPost, c.serverURL+"/api/v1/upload",
		bytes.NewReader(payload))
	if err != nil {
		return 0, 0
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed <= 0 || resp.StatusCode != http.StatusOK {
		return 0, totalBytes
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes
}
