package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

const (
	pingPath             = "/api/v1/ping"
	downloadPath         = "/api/v1/download"
	uploadPath           = "/api/v1/upload"
	downloadDirection    = "download"
	uploadDirection      = "upload"
	engineDuration       = 1 * time.Second
	engineTimeout        = 5 * time.Second
	runContextTimeout    = 2 * time.Second
	downloadChunkSize    = 65536
	multiStreamChunkSize = 64 * 1024
	measureHTTPPingFmt   = "measureHTTPPing: %v"
)

func TestHTTPTestEngineDownload(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc(downloadPath, handler.Download)
	mux.HandleFunc(pingPath, handler.Ping)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       engineDuration,
		Streams:        1,
		ChunkSize:      downloadChunkSize,
		Direction:      downloadDirection,
		GraceTime:      0,
		StreamDelay:    0,
		OverheadFactor: 1.0,
		Timeout:        engineTimeout,
	}

	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new http test engine: %v", err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), runContextTimeout)
	t.Cleanup(cancel)

	if err := engine.Run(ctx); err != nil {
		t.Fatalf("run download: %v", err)
	}

	metrics := engine.GetMetrics()
	if metrics.BytesTransferred == 0 {
		t.Fatalf("expected bytes transferred > 0")
	}
	if metrics.BytesReceived == 0 {
		t.Fatalf("expected bytes received > 0")
	}
}

func TestHTTPTestEngineDownloadUsesGlobalDuration(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc(downloadPath, handler.Download)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       engineDuration,
		Streams:        8,
		ChunkSize:      downloadChunkSize,
		Direction:      downloadDirection,
		GraceTime:      0,
		StreamDelay:    200 * time.Millisecond,
		OverheadFactor: 1.0,
		Timeout:        engineTimeout,
	}

	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new http test engine: %v", err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	start := time.Now()
	if err := engine.Run(ctx); err != nil {
		t.Fatalf("run download: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > engineDuration+700*time.Millisecond {
		t.Fatalf("download exceeded global duration: elapsed=%s duration=%s", elapsed, engineDuration)
	}
	if metrics := engine.GetMetrics(); metrics.BytesTransferred == 0 {
		t.Fatalf("expected bytes transferred > 0")
	}
}

func TestHTTPAddBytesStopsAtConfiguredDuration(t *testing.T) {
	engine := &HTTPTestEngine{config: &HTTPTestConfig{Duration: engineDuration}}

	engine.addBytes(100, engineDuration)
	if total := atomic.LoadInt64(&engine.totalBytes); total != 0 {
		t.Fatalf("total bytes after duration = %d, want 0", total)
	}

	engine.addBytes(100, engineDuration-time.Millisecond)
	if total := atomic.LoadInt64(&engine.totalBytes); total != 100 {
		t.Fatalf("total bytes before duration = %d, want 100", total)
	}
}

func TestHTTPTestEngineUpload(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc(uploadPath, handler.Upload)
	mux.HandleFunc(pingPath, handler.Ping)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       engineDuration,
		Streams:        1,
		ChunkSize:      downloadChunkSize,
		Direction:      uploadDirection,
		GraceTime:      0,
		StreamDelay:    0,
		OverheadFactor: 1.0,
		Timeout:        engineTimeout,
	}

	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new http test engine: %v", err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), runContextTimeout)
	t.Cleanup(cancel)

	if err := engine.Run(ctx); err != nil {
		t.Fatalf("run upload: %v", err)
	}

	metrics := engine.GetMetrics()
	if metrics.BytesTransferred == 0 {
		t.Fatalf("expected bytes transferred > 0")
	}
	if metrics.BytesSent == 0 {
		t.Fatalf("expected bytes sent > 0")
	}
}

func TestMeasureHTTPPingReturnsSamples(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pong":true}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), runContextTimeout)
	defer cancel()

	samples, err := measureHTTPPing(ctx, server.URL, 3)
	if err != nil {
		t.Fatalf(measureHTTPPingFmt, err)
	}
	if len(samples) != 3 {
		t.Fatalf("samples len = %d, want 3", len(samples))
	}
}

func TestMeasureHTTPPingHonorsContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	samples, err := measureHTTPPing(ctx, server.URL, 5)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf(measureHTTPPingFmt, err)
	}
	if len(samples) != 0 {
		t.Fatalf("samples len = %d, want 0", len(samples))
	}
	if elapsed > 2*time.Second {
		t.Fatalf("ping should return promptly on context cancel, elapsed=%s", elapsed)
	}
}

func TestMeasureHTTPPingSkipsNonOKResponses(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"busy"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), runContextTimeout)
	defer cancel()

	samples, err := measureHTTPPing(ctx, server.URL, 3)
	if err != nil {
		t.Fatalf(measureHTTPPingFmt, err)
	}
	if len(samples) != 0 {
		t.Fatalf("samples len = %d, want 0 for non-OK responses", len(samples))
	}
}

func TestMeasureHTTPPingAllFailuresReturnEmptyNoError(t *testing.T) {
	// Reserve an ephemeral port, then close it so subsequent requests fail.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	baseURL := "http://" + ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	samples, err := measureHTTPPing(ctx, baseURL, 3)
	if err != nil {
		t.Fatalf(measureHTTPPingFmt, err)
	}
	if len(samples) != 0 {
		t.Fatalf("samples len = %d, want 0 when all requests fail", len(samples))
	}
}

func TestHTTPEngineMultiStreamFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(downloadPath, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "overloaded", http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       engineDuration,
		Streams:        3,
		ChunkSize:      multiStreamChunkSize,
		Direction:      downloadDirection,
		GraceTime:      0,
		StreamDelay:    0,
		OverheadFactor: 1.0,
		Timeout:        runContextTimeout,
	}
	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	defer engine.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = engine.Run(ctx)
	if err == nil {
		t.Fatal("expected failure when all streams fail")
	}
	if got := err.Error(); got == "" || !containsAll(got, "download streams failed", "download failed") {
		t.Fatalf("unexpected error text: %q", got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
