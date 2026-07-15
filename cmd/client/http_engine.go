package client

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// clientBufferSize sizes the download read buffer. 1 MiB measured ~5% faster
// than 64 KiB on multi-Gbit/s loopback runs (fewer Read calls per second).
const clientBufferSize = 1024 * 1024

type HTTPTestConfig struct {
	ServerURL   string
	Duration    time.Duration
	Streams     int
	ChunkSize   int
	Direction   string
	GraceTime   time.Duration
	StreamDelay time.Duration
	Timeout     time.Duration
}

func newClientBuffer() *[]byte {
	buf := make([]byte, clientBufferSize)
	return &buf
}

type HTTPTestEngine struct {
	config        *HTTPTestConfig
	client        *http.Client
	startUnixNano int64
	totalBytes    int64
	uploadPayload []byte
	bufferPool    sync.Pool
}

func NewHTTPTestEngine(cfg *HTTPTestConfig) (*HTTPTestEngine, error) {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DisableCompression:  true,
		DialContext:         dialer.DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: max(8, cfg.Streams*2),
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
	payloadSize := max(cfg.ChunkSize, 4*1024*1024)
	payload := make([]byte, payloadSize)
	if _, err := rand.Read(payload); err != nil {
		return nil, fmt.Errorf("generate upload payload: %w", err)
	}

	return &HTTPTestEngine{
		config:        cfg,
		client:        client,
		uploadPayload: payload,
		bufferPool: sync.Pool{
			New: func() any {
				return newClientBuffer()
			},
		},
	}, nil
}

func (e *HTTPTestEngine) Run(ctx context.Context) error {
	atomic.StoreInt64(&e.startUnixNano, time.Now().UnixNano())

	switch e.config.Direction {
	case "download":
		return e.runDownload(ctx)
	case "upload":
		return e.runUpload(ctx)
	default:
		return fmt.Errorf("unsupported http direction: %s", e.config.Direction)
	}
}

func (e *HTTPTestEngine) GetMetrics() EngineMetrics {
	elapsed := e.elapsedSinceStart()
	totalBytes := atomic.LoadInt64(&e.totalBytes)
	throughputMbps := 0.0
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
	}
	return EngineMetrics{
		ThroughputMbps:   throughputMbps,
		BytesTransferred: totalBytes,
	}
}

func (e *HTTPTestEngine) elapsedSinceStart() time.Duration {
	startUnixNano := atomic.LoadInt64(&e.startUnixNano)
	if startUnixNano == 0 {
		return 0
	}
	return time.Since(time.Unix(0, startUnixNano))
}

func (e *HTTPTestEngine) Close() {
	e.client.CloseIdleConnections()
}
