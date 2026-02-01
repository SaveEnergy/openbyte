package client

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func runStream(ctx context.Context, config *Config, formatter OutputFormatter, streamID *string) error {
	if config.Protocol == "http" {
		return runHTTPStream(ctx, config, formatter)
	}
	streamResp, err := startStream(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start stream: %v\n\n"+
			"Troubleshooting:\n"+
			"  - Check server is running: curl %s/health\n"+
			"  - Verify server URL: openbyte client --server-url %s\n"+
			"  - Check network connectivity", err, config.ServerURL, config.ServerURL)
	}

	*streamID = streamResp.StreamID

	if streamResp.Mode == "client" {
		return runClientSideTest(ctx, config, formatter, streamResp)
	}

	return streamMetrics(ctx, streamResp.WebSocketURL, formatter, config)
}

// EngineRunner interface for TCP/UDP engines
type EngineRunner interface {
	Run(ctx context.Context) error
	GetMetrics() EngineMetrics
	IsRunning() bool
}

func runClientSideTest(ctx context.Context, config *Config, formatter OutputFormatter, streamResp *StreamResponse) error {
	testAddr := streamResp.TestServerTCP
	if config.Protocol == "udp" {
		testAddr = streamResp.TestServerUDP
	}

	if testAddr == "" {
		return fmt.Errorf("server did not provide test server address")
	}

	engineCfg := &TestEngineConfig{
		ServerAddr: testAddr,
		Protocol:   config.Protocol,
		Direction:  config.Direction,
		Duration:   time.Duration(config.Duration) * time.Second,
		Streams:    config.Streams,
		PacketSize: config.PacketSize,
		WarmUp:     time.Duration(config.WarmUp) * time.Second,
	}

	engine := NewTestEngine(engineCfg)

	startTime := time.Now()
	testCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- engine.Run(testCtx)
	}()

	metricsTicker := time.NewTicker(500 * time.Millisecond)
	defer metricsTicker.Stop()

	var lastMetrics EngineMetrics

	for {
		select {
		case err := <-doneCh:
			lastMetrics = engine.GetMetrics()
			results := buildResults(streamResp.StreamID, config, lastMetrics, startTime)
			formatter.FormatComplete(results)

			completeStream(config, streamResp.StreamID, lastMetrics)

			if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			return nil

		case <-metricsTicker.C:
			lastMetrics = engine.GetMetrics()
			elapsed := time.Since(startTime)
			progress := (elapsed.Seconds() / float64(config.Duration)) * 100
			if progress > 100 {
				progress = 100
			}
			remaining := float64(config.Duration) - elapsed.Seconds()
			if remaining < 0 {
				remaining = 0
			}

			formatter.FormatProgress(progress, elapsed.Seconds(), remaining)

			m := &types.Metrics{
				ThroughputMbps:    lastMetrics.ThroughputMbps,
				ThroughputAvgMbps: lastMetrics.ThroughputMbps,
				BytesTransferred:  lastMetrics.BytesTransferred,
				Latency: types.LatencyMetrics{
					MinMs: lastMetrics.Latency.MinMs,
					MaxMs: lastMetrics.Latency.MaxMs,
					AvgMs: lastMetrics.Latency.AvgMs,
					P50Ms: lastMetrics.Latency.P50Ms,
					P95Ms: lastMetrics.Latency.P95Ms,
					P99Ms: lastMetrics.Latency.P99Ms,
					Count: lastMetrics.Latency.Count,
				},
				JitterMs:  lastMetrics.JitterMs,
				Timestamp: time.Now(),
			}
			formatter.FormatMetrics(m)

		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}
}

func runHTTPStream(ctx context.Context, config *Config, formatter OutputFormatter) error {
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	pingSamples, _ := measureHTTPPing(pingCtx, config.ServerURL, 20)
	pingCancel()

	latencyStats := calculateClientLatency(pingSamples)
	jitter := calculateClientJitter(pingSamples)

	graceTime := 1500 * time.Millisecond
	if config.Direction == "upload" {
		graceTime = 3000 * time.Millisecond
	}

	httpCfg := &HTTPTestConfig{
		ServerURL:      config.ServerURL,
		Duration:       time.Duration(config.Duration) * time.Second,
		Streams:        config.Streams,
		ChunkSize:      config.ChunkSize,
		Direction:      config.Direction,
		GraceTime:      graceTime,
		StreamDelay:    200 * time.Millisecond,
		OverheadFactor: 1.06,
		APIKey:         config.APIKey,
		Timeout:        time.Duration(config.Timeout) * time.Second,
	}

	engine := NewHTTPTestEngine(httpCfg)

	startTime := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, httpCfg.Duration)
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- engine.Run(testCtx)
	}()

	metricsTicker := time.NewTicker(500 * time.Millisecond)
	defer metricsTicker.Stop()

	for {
		select {
		case err := <-doneCh:
			metrics := engine.GetMetrics()
			metrics.Latency = latencyStats
			metrics.JitterMs = jitter

			totalBytes := metrics.BytesTransferred
			measuredElapsed := time.Since(startTime)
			if measuredElapsed > httpCfg.Duration {
				measuredElapsed = httpCfg.Duration
			}
			measuredElapsed -= graceTime
			if measuredElapsed <= 0 {
				measuredElapsed = 1 * time.Millisecond
			}
			avgSpeed := float64(totalBytes*8) / measuredElapsed.Seconds() / 1_000_000 * httpCfg.OverheadFactor
			metrics.ThroughputMbps = avgSpeed

			httpConfig := *config
			httpConfig.PacketSize = config.ChunkSize
			results := buildResults("http", &httpConfig, metrics, startTime)
			formatter.FormatComplete(results)

			if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			return nil

		case <-metricsTicker.C:
			metrics := engine.GetMetrics()
			elapsed := time.Since(startTime)
			progress := (elapsed.Seconds() / float64(config.Duration)) * 100
			if progress > 100 {
				progress = 100
			}
			remaining := float64(config.Duration) - elapsed.Seconds()
			if remaining < 0 {
				remaining = 0
			}

			formatter.FormatProgress(progress, elapsed.Seconds(), remaining)

			m := &types.Metrics{
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
			formatter.FormatMetrics(m)

		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}
}

func buildResults(streamID string, config *Config, metrics EngineMetrics, startTime time.Time) *StreamResults {
	endTime := time.Now()
	return &StreamResults{
		StreamID: streamID,
		Status:   "completed",
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
		StartTime:       startTime.Format(time.RFC3339),
		EndTime:         endTime.Format(time.RFC3339),
		DurationSeconds: endTime.Sub(startTime).Seconds(),
	}
}

func createFormatter(config *Config) OutputFormatter {
	if config.JSON {
		return &JSONFormatter{writer: os.Stdout}
	}
	if config.Plain {
		return NewPlainFormatter(os.Stdout, config.Verbose, config.NoColor)
	}
	return NewInteractiveFormatter(os.Stdout, config.Verbose, config.NoColor)
}

