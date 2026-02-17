package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
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

func (e *HTTPTestEngine) runDownload(ctx context.Context) error {
	reqURL := e.buildDownloadURL()
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			if err := e.runDownloadStream(ctx, reqURL, delay); err != nil {
				errCh <- err
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	close(errCh)
	return joinStreamErrors("download", errCh)
}

func (e *HTTPTestEngine) runDownloadStream(ctx context.Context, reqURL string, delay time.Duration) error {
	time.Sleep(delay)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	if e.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		drainAndClose(resp)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return e.handleNonOKResponse(ctx, "download", resp)
	}

	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
	defer e.bufferPool.Put(buf)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			atomic.AddInt64(&e.bytesReceived, int64(n))
			e.addBytes(int64(n), e.elapsedSinceStart())
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return readErr
		}
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
			if err := e.runUploadStream(ctx, reqURL, deadline, delay); err != nil {
				errCh <- err
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	close(errCh)
	return joinStreamErrors("upload", errCh)
}

func (e *HTTPTestEngine) runUploadStream(ctx context.Context, reqURL string, deadline time.Time, delay time.Duration) error {
	time.Sleep(delay)
	now := time.Now()
	for now.Before(deadline) && ctx.Err() == nil {
		reader := bytes.NewReader(e.uploadPayload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		if e.config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
		}

		resp, err := e.client.Do(req)
		if err != nil {
			drainAndClose(resp)
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return e.handleNonOKResponse(ctx, "upload", resp)
		}
		drainAndClose(resp)

		atomic.AddInt64(&e.bytesSent, int64(len(e.uploadPayload)))
		e.addBytes(int64(len(e.uploadPayload)), e.elapsedSinceStart())
		now = time.Now()
	}
	return nil
}

func joinStreamErrors(direction string, errCh <-chan error) error {
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s streams failed: %w", direction, errors.Join(errs...))
}

func drainAndClose(resp *http.Response) {
	if resp == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func (e *HTTPTestEngine) handleNonOKResponse(ctx context.Context, direction string, resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		wait := parseRetryAfter(resp.Header.Get("Retry-After"), time.Second)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
		}
	}
	drainAndClose(resp)
	return fmt.Errorf("%s failed: %s", direction, resp.Status)
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
	base := strings.TrimRight(e.config.ServerURL, "/")
	u, err := url.Parse(base + "/api/v1/download")
	if err != nil {
		return base + "/api/v1/download"
	}
	q := u.Query()
	q.Set("duration", fmt.Sprintf("%d", int(e.config.Duration.Seconds())))
	q.Set("chunk", fmt.Sprintf("%d", e.config.ChunkSize))
	u.RawQuery = q.Encode()
	return u.String()
}

func (e *HTTPTestEngine) buildUploadURL() string {
	return strings.TrimRight(e.config.ServerURL, "/") + "/api/v1/upload"
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

func parseRetryAfter(value string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	sec, err := strconv.Atoi(trimmed)
	if err != nil || sec < 1 {
		return fallback
	}
	d := time.Duration(sec) * time.Second
	if d > 2*time.Minute {
		return 2 * time.Minute
	}
	return d
}

func measureHTTPPing(ctx context.Context, serverURL string, samples int) ([]time.Duration, error) {
	if samples <= 0 {
		return nil, nil
	}
	pingURL := strings.TrimRight(serverURL, "/") + "/api/v1/ping"
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
	defer client.CloseIdleConnections()
	results := make([]time.Duration, 0, samples)

	for range samples {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			return results, err
		}
		resp, err := client.Do(req)
		if err != nil {
			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			results = append(results, time.Since(start))
		}
	}
	return results, nil
}
