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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

const (
	directionDownload = "download"
	directionUpload   = "upload"
	pathHealth        = "/health"
	pathPing          = "/api/v1/ping"
	pathDownload      = "/api/v1/download"
	pathUpload        = "/api/v1/upload"
	authBearerPrefix  = "Bearer "
)

var (
	ErrLatencyMeasurementFailed  = errors.New("latency measurement failed")
	ErrDownloadMeasurementFailed = errors.New("download measurement failed")
	ErrUploadMeasurementFailed   = errors.New("upload measurement failed")
)

// Client is an openByte speed test client targeting a single server.
// It is safe for concurrent use as long as options are not mutated after New().
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

const defaultHTTPTimeout = 60 * time.Second

// New creates a new openByte client targeting the given server URL.
// Returned client should be treated as immutable after construction.
// Default http.Client has a 60s timeout to avoid indefinite hangs on stalled connections.
func New(serverURL string, opts ...Option) *Client {
	c := &Client{
		serverURL: strings.TrimRight(serverURL, "/"),
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
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

// --- Internal helpers ---

func (c *Client) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+pathHealth, nil)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
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

func (c *Client) measureLatency(ctx context.Context, samples int) (avgMs, jitterMs float64, ok bool) {
	latencies := c.collectLatencySamples(ctx, samples)
	if len(latencies) < 2 {
		return 0, 0, false
	}
	avgMs = float64(latenciesSum(latencies)) / float64(len(latencies)) / float64(time.Millisecond)
	jitterMs = jitterFromLatencies(latencies)
	return avgMs, jitterMs, true
}

func (c *Client) collectLatencySamples(ctx context.Context, samples int) []time.Duration {
	pingURL := c.serverURL + pathPing
	var latencies []time.Duration
	for range samples {
		if ctx.Err() != nil {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			continue
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
		}
		latency, sampleOK := c.latencySample(req, time.Now())
		if sampleOK {
			latencies = append(latencies, latency)
		}
	}
	return latencies
}

func latenciesSum(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total
}

func jitterFromLatencies(latencies []time.Duration) float64 {
	var jitterSum float64
	for i := 1; i < len(latencies); i++ {
		diff := latencies[i] - latencies[i-1]
		if diff < 0 {
			diff = -diff
		}
		jitterSum += float64(diff) / float64(time.Millisecond)
	}
	return jitterSum / float64(len(latencies)-1)
}

func (c *Client) latencySample(req *http.Request, start time.Time) (time.Duration, bool) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}
	return time.Since(start), true
}

func (c *Client) downloadBurst(ctx context.Context, durationSec int) (float64, bool) {
	mbps, _, ok := c.downloadMeasured(ctx, durationSec)
	return mbps, ok
}

func (c *Client) downloadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64, ok bool) {
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s%s?duration=%d&chunk=1048576", c.serverURL, pathDownload, durationSec)
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, false
	}
	req.Header.Set("Accept-Encoding", "identity")
	if c.apiKey != "" {
		req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return 0, 0, false
	}

	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		totalBytes += int64(n)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return 0, totalBytes, false
			}
			break
		}
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0, totalBytes, false
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes, true
}

func (c *Client) uploadBurst(ctx context.Context, durationSec int) (float64, bool) {
	mbps, _, ok := c.uploadMeasured(ctx, durationSec)
	return mbps, ok
}

func (c *Client) uploadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64, ok bool) {
	upCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()
	totalBytes, elapsed := c.uploadLoop(upCtx, durationSec)
	if elapsed <= 0 || totalBytes == 0 {
		return 0, totalBytes, false
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes, true
}

func (c *Client) uploadLoop(ctx context.Context, durationSec int) (totalBytes int64, elapsed time.Duration) {
	payload := make([]byte, 1024*1024)
	targetDuration := time.Duration(durationSec) * time.Second
	start := time.Now()
	iterations := 0
	for {
		if ctx.Err() != nil {
			break
		}
		if iterations > 0 && time.Since(start) >= targetDuration {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+pathUpload,
			bytes.NewReader(payload))
		if err != nil {
			return totalBytes, 0
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		if c.apiKey != "" {
			req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return totalBytes, 0
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return totalBytes, 0
		}
		totalBytes += int64(len(payload))
		iterations++
	}
	return totalBytes, time.Since(start)
}
