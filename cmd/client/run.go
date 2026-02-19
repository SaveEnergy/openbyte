package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
	"github.com/saveenergy/openbyte/pkg/types"
)

func runStream(ctx context.Context, config *Config, formatter OutputFormatter, streamID *atomic.Value) error {
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

	streamID.Store(streamResp.StreamID)

	if streamResp.Mode == "client" {
		return runClientSideTest(ctx, config, formatter, streamResp)
	}

	if err := streamMetrics(ctx, streamResp.WebSocketURL, formatter, config); err != nil {
		return cancelStreamWithCleanup(ctx, config, streamResp.StreamID, err)
	}
	return nil
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

	totalRunTime := float64(config.WarmUp + config.Duration)
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
			return handleClientTestCompletion(ctx, config, formatter, streamResp.StreamID, startTime, engine.GetMetrics(), err)

		case <-metricsTicker.C:
			lastMetrics = engine.GetMetrics()
			elapsed := time.Since(startTime)
			progress, remaining := computeProgress(elapsed, totalRunTime)
			if err := emitProgressAndMetrics(
				formatter,
				engineMetricsToTypesMetrics(lastMetrics),
				progress,
				elapsed.Seconds(),
				remaining,
			); err != nil {
				cancel()
				return err
			}

		case <-ctx.Done():
			cancel()
			return cancelStreamWithCleanup(ctx, config, streamResp.StreamID, ctx.Err())
		}
	}
}

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
		return fmt.Errorf("formatter output failed: %w", ferr)
	}

	completeErr := completeStream(ctx, config, streamID, metrics)
	if completeErr != nil {
		if runErr != nil && !errors.Is(runErr, context.DeadlineExceeded) && !errors.Is(runErr, context.Canceled) {
			return fmt.Errorf("%v (and completion report failed: %v)", runErr, completeErr)
		}
		return fmt.Errorf("failed to report completion: %w", completeErr)
	}

	if runErr != nil && !errors.Is(runErr, context.DeadlineExceeded) && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return nil
}

func runHTTPStream(ctx context.Context, config *Config, formatter OutputFormatter) error {
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	pingSamples, pingErr := measureHTTPPing(pingCtx, config.ServerURL, 20)
	pingCancel()

	latencyStats, jitter := computePingMetrics(pingSamples)
	if pingErr != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: warning: ping sampling failed: %v\n", pingErr)
	}
	if len(pingSamples) == 0 {
		fmt.Fprintln(os.Stderr, "openbyte client: warning: ping sampling returned no successful samples; latency/jitter set to 0")
	}

	graceTime := time.Duration(config.WarmUp) * time.Second

	httpCfg := &HTTPTestConfig{
		ServerURL:      config.ServerURL,
		Duration:       time.Duration(config.Duration) * time.Second,
		Streams:        config.Streams,
		ChunkSize:      config.ChunkSize,
		Direction:      config.Direction,
		GraceTime:      graceTime,
		StreamDelay:    200 * time.Millisecond,
		OverheadFactor: 1.0,
		APIKey:         config.APIKey,
		Timeout:        time.Duration(config.Timeout) * time.Second,
	}
	minTimeout := httpCfg.Duration + 10*time.Second
	if httpCfg.Timeout < minTimeout {
		httpCfg.Timeout = minTimeout
	}

	engine, err := NewHTTPTestEngine(httpCfg)
	if err != nil {
		return fmt.Errorf("init http engine: %w", err)
	}
	defer engine.Close()

	startTime := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, httpCfg.Duration+10*time.Second)
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
			return finalizeHTTPStreamRun(finalizeHTTPStreamRunInput{
				config:       config,
				httpCfg:      httpCfg,
				formatter:    formatter,
				engine:       engine,
				startTime:    startTime,
				graceTime:    graceTime,
				latencyStats: latencyStats,
				jitter:       jitter,
				runErr:       err,
			})

		case <-metricsTicker.C:
			metrics := engine.GetMetrics()
			elapsed := time.Since(startTime)
			progress, remaining := computeProgress(elapsed, float64(config.Duration))
			if err := emitProgressAndMetrics(
				formatter,
				engineMetricsToTypesMetrics(metrics),
				progress,
				elapsed.Seconds(),
				remaining,
			); err != nil {
				cancel()
				return err
			}

		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}
}

func cancelStreamWithCleanup(ctx context.Context, config *Config, streamID string, rootErr error) error {
	cancelErr := CancelStream(ctx, config.ServerURL, streamID, config.APIKey)
	if cancelErr != nil {
		return fmt.Errorf("%v (and cancel cleanup failed: %v)", rootErr, cancelErr)
	}
	return rootErr
}

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
		return fmt.Errorf("formatter output failed: %w", ferr)
	}
	return nil
}

type finalizeHTTPStreamRunInput struct {
	config       *Config
	httpCfg      *HTTPTestConfig
	formatter    OutputFormatter
	engine       *HTTPTestEngine
	startTime    time.Time
	graceTime    time.Duration
	latencyStats LatencyStats
	jitter       float64
	runErr       error
}

func finalizeHTTPStreamRun(input finalizeHTTPStreamRunInput) error {
	metrics := input.engine.GetMetrics()
	metrics.Latency = input.latencyStats
	metrics.JitterMs = input.jitter

	totalBytes := metrics.BytesTransferred
	measuredElapsed := min(time.Since(input.startTime), input.httpCfg.Duration)
	measuredElapsed -= input.graceTime
	if measuredElapsed <= 0 {
		measuredElapsed = 1 * time.Millisecond
	}
	avgSpeed := float64(totalBytes*8) / measuredElapsed.Seconds() / 1_000_000 * input.httpCfg.OverheadFactor
	metrics.ThroughputMbps = avgSpeed

	httpConfig := *input.config
	httpConfig.PacketSize = input.config.ChunkSize
	results := buildResults("http", &httpConfig, metrics, input.startTime)
	input.formatter.FormatComplete(results)
	if ferr := formatterLastError(input.formatter); ferr != nil {
		return fmt.Errorf("formatter output failed: %w", ferr)
	}

	if input.runErr != nil && !errors.Is(input.runErr, context.DeadlineExceeded) && !errors.Is(input.runErr, context.Canceled) {
		return input.runErr
	}
	return nil
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
	case "download":
		downMbps = throughput
	case "upload":
		upMbps = throughput
	case "bidirectional":
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
		Status:        "completed",
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
