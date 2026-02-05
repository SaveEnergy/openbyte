package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	startTime     time.Time
	totalBytes    int64
	graceBytes    int64
	graceDone     int32
	bytesSent     int64
	bytesReceived int64
	running       int32
	uploadPayload []byte
	bufferPool    sync.Pool
}

func NewHTTPTestEngine(cfg *HTTPTestConfig) *HTTPTestEngine {
	transport := &http.Transport{
		DisableCompression: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}
	payloadSize := cfg.ChunkSize
	if payloadSize < 4*1024*1024 {
		payloadSize = 4 * 1024 * 1024
	}
	payload := make([]byte, payloadSize)
	_, _ = rand.Read(payload)

	return &HTTPTestEngine{
		config:        cfg,
		client:        client,
		uploadPayload: payload,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024)
			},
		},
	}
}

func (e *HTTPTestEngine) Run(ctx context.Context) error {
	atomic.StoreInt32(&e.running, 1)
	defer atomic.StoreInt32(&e.running, 0)
	e.startTime = time.Now()

	switch e.config.Direction {
	case "download":
		return e.runDownload(ctx)
	case "upload":
		return e.runUpload(ctx)
	default:
		return fmt.Errorf("unsupported http direction: %s", e.config.Direction)
	}
}

func (e *HTTPTestEngine) runDownload(ctx context.Context) error {
	reqURL := e.buildDownloadURL()
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			time.Sleep(delay)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if err != nil {
				errCh <- err
				return
			}
			req.Header.Set("Accept-Encoding", "identity")
			if e.config.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
			}

			resp, err := e.client.Do(req)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("download failed: %s", resp.Status)
				return
			}

			buf := e.bufferPool.Get().([]byte)
			defer e.bufferPool.Put(buf)

			for {
				n, err := resp.Body.Read(buf)
				if n > 0 {
					atomic.AddInt64(&e.bytesReceived, int64(n))
					e.addBytes(int64(n), time.Since(e.startTime))
				}
				if err != nil {
					if err == io.EOF || ctx.Err() != nil {
						return
					}
					errCh <- err
					return
				}
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (e *HTTPTestEngine) runUpload(ctx context.Context) error {
	reqURL := e.buildUploadURL()
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)
	deadline := time.Now().Add(e.config.Duration)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			time.Sleep(delay)

			for time.Now().Before(deadline) && ctx.Err() == nil {
				reader := bytes.NewReader(e.uploadPayload)
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, reader)
				if err != nil {
					errCh <- err
					return
				}
				req.Header.Set("Content-Type", "application/octet-stream")
				if e.config.APIKey != "" {
					req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
				}

				resp, err := e.client.Do(req)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					errCh <- err
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errCh <- fmt.Errorf("upload failed: %s", resp.Status)
					return
				}

				atomic.AddInt64(&e.bytesSent, int64(len(e.uploadPayload)))
				e.addBytes(int64(len(e.uploadPayload)), time.Since(e.startTime))
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (e *HTTPTestEngine) addBytes(n int64, elapsed time.Duration) {
	if elapsed < e.config.GraceTime {
		atomic.AddInt64(&e.graceBytes, n)
		return
	}
	if atomic.CompareAndSwapInt32(&e.graceDone, 0, 1) {
		atomic.StoreInt64(&e.totalBytes, 0)
	}
	atomic.AddInt64(&e.totalBytes, n)
}

func (e *HTTPTestEngine) buildDownloadURL() string {
	base := stringsTrimSuffix(e.config.ServerURL)
	u, _ := url.Parse(base + "/api/v1/download")
	q := u.Query()
	q.Set("duration", fmt.Sprintf("%d", int(e.config.Duration.Seconds())))
	q.Set("chunk", fmt.Sprintf("%d", e.config.ChunkSize))
	u.RawQuery = q.Encode()
	return u.String()
}

func (e *HTTPTestEngine) buildUploadURL() string {
	return stringsTrimSuffix(e.config.ServerURL) + "/api/v1/upload"
}

func (e *HTTPTestEngine) GetMetrics() EngineMetrics {
	elapsed := time.Since(e.startTime)
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

func (e *HTTPTestEngine) IsRunning() bool {
	return atomic.LoadInt32(&e.running) == 1
}

func measureHTTPPing(ctx context.Context, serverURL string, samples int) ([]time.Duration, error) {
	if samples <= 0 {
		return nil, nil
	}
	pingURL := stringsTrimSuffix(serverURL) + "/api/v1/ping"
	client := &http.Client{Timeout: 10 * time.Second}
	results := make([]time.Duration, 0, samples)

	for i := 0; i < samples; i++ {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			return results, err
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		results = append(results, time.Since(start))
	}
	return results, nil
}

func stringsTrimSuffix(input string) string {
	return strings.TrimRight(input, "/")
}
