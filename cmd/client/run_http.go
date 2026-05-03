package client

import (
	"context"
	"fmt"
	"os"
	"time"
)

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
			if err := emitCurrentProgress(formatter, startTime, float64(config.Duration), engine.GetMetrics()); err != nil {
				cancel()
				return err
			}

		case <-ctx.Done():
			cancel()
			return ctx.Err()
		}
	}
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
	results := buildResults(protocolHTTP, &httpConfig, metrics, input.startTime)
	input.formatter.FormatComplete(results)
	if ferr := formatterLastError(input.formatter); ferr != nil {
		return fmt.Errorf(formatterOutputFailedFmt, ferr)
	}

	if shouldReturnRunError(input.runErr) {
		return input.runErr
	}
	return nil
}
