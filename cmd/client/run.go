package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func runStream(ctx context.Context, config *Config, formatter OutputFormatter, streamID *string) error {
	streamResp, err := startStream(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start stream: %v\n\n"+
			"Troubleshooting:\n"+
			"  - Check server is running: curl %s/health\n"+
			"  - Verify server URL: obyte --server-url %s\n"+
			"  - Check network connectivity", err, config.ServerURL, config.ServerURL)
	}

	*streamID = streamResp.StreamID

	if streamResp.Mode == "client" {
		return runClientSideTest(ctx, config, formatter, streamResp)
	}

	return streamMetrics(ctx, streamResp.WebSocketURL, formatter, config)
}

// EngineRunner interface for both TCP/UDP and QUIC engines
type EngineRunner interface {
	Run(ctx context.Context) error
	GetMetrics() EngineMetrics
	IsRunning() bool
}

func runClientSideTest(ctx context.Context, config *Config, formatter OutputFormatter, streamResp *StreamResponse) error {
	testAddr := streamResp.TestServerTCP
	if config.Protocol == "udp" {
		testAddr = streamResp.TestServerUDP
	} else if config.Protocol == "quic" {
		// For QUIC, derive address from server URL (use QUIC port 8083)
		testAddr = streamResp.TestServerQUIC
		if testAddr == "" {
			// Fallback: derive from TCP address, replace port with 8083
			testAddr = deriveQUICAddress(streamResp.TestServerTCP)
		}
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

	var engine EngineRunner
	if config.Protocol == "quic" {
		engine = NewQUICEngine(engineCfg)
	} else {
		engine = NewTestEngine(engineCfg)
	}

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

// deriveQUICAddress derives QUIC address from TCP address (replaces port with 8083)
func deriveQUICAddress(tcpAddr string) string {
	if tcpAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(tcpAddr)
	if err != nil {
		return tcpAddr + ":8083"
	}
	return net.JoinHostPort(host, "8083")
}
