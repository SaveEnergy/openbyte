package client

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

func runStream(ctx context.Context, config *Config, formatter OutputFormatter, streamID *atomic.Value) error {
	if config.Protocol == protocolHTTP {
		return runHTTPStream(ctx, config, formatter)
	}
	streamResp, err := startStream(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start stream: %w\n\n"+
			"Troubleshooting:\n"+
			"  - Check server is running: curl %s/health\n"+
			"  - Verify server URL: openbyte client --server-url %s\n"+
			"  - Check network connectivity", err, config.ServerURL, config.ServerURL)
	}

	streamID.Store(streamResp.StreamID)

	if streamResp.Mode == modeClient {
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
	if config.Protocol == protocolUDP {
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

	for {
		select {
		case err := <-doneCh:
			return handleClientTestCompletion(ctx, config, formatter, streamResp.StreamID, startTime, engine.GetMetrics(), err)

		case <-metricsTicker.C:
			if err := emitCurrentProgress(formatter, startTime, totalRunTime, engine.GetMetrics()); err != nil {
				cancel()
				return cancelStreamWithCleanup(ctx, config, streamResp.StreamID, err)
			}

		case <-ctx.Done():
			cancel()
			return cancelStreamWithCleanup(ctx, config, streamResp.StreamID, ctx.Err())
		}
	}
}
