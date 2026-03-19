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

type HTTPTestConfig struct {
	ServerURL      string
	Duration       time.Duration
	Streams        int
	ChunkSize      int
	Direction      string
	GraceTime      time.Duration
	StreamDelay    time.Duration
	OverheadFactor float64
	APIKey         string
	Timeout        time.Duration
}

type HTTPTestEngine struct {
	config        *HTTPTestConfig
	client        *http.Client
	startUnixNano int64
	totalBytes    int64
	graceBytes    int64
	graceDone     int32
	bytesSent     int64
	bytesReceived int64
	running       int32
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
				return make([]byte, 64*1024)
			},
		},
	}, nil
}

func (e *HTTPTestEngine) Run(ctx context.Context) error {
	atomic.StoreInt32(&e.running, 1)
	defer atomic.StoreInt32(&e.running, 0)
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
	bytesSent := atomic.LoadInt64(&e.bytesSent)
	bytesRecv := atomic.LoadInt64(&e.bytesReceived)
	throughputMbps := 0.0
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
	}
	return EngineMetrics{
		ThroughputMbps:   throughputMbps,
		BytesTransferred: totalBytes,
		BytesSent:        bytesSent,
		BytesReceived:    bytesRecv,
		Elapsed:          elapsed,
		Running:          atomic.LoadInt32(&e.running) == 1,
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

func (e *HTTPTestEngine) IsRunning() bool {
	return atomic.LoadInt32(&e.running) == 1
}
