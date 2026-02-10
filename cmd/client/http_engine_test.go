package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestHTTPTestEngineDownload(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download", handler.Download)
	mux.HandleFunc("/api/v1/ping", handler.Ping)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       1 * time.Second,
		Streams:        1,
		ChunkSize:      65536,
		Direction:      "download",
		GraceTime:      0,
		StreamDelay:    0,
		OverheadFactor: 1.0,
		Timeout:        5 * time.Second,
	}

	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new http test engine: %v", err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

func TestHTTPTestEngineUpload(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/upload", handler.Upload)
	mux.HandleFunc("/api/v1/ping", handler.Ping)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := &HTTPTestConfig{
		ServerURL:      server.URL,
		Duration:       1 * time.Second,
		Streams:        1,
		ChunkSize:      65536,
		Direction:      "upload",
		GraceTime:      0,
		StreamDelay:    0,
		OverheadFactor: 1.0,
		Timeout:        5 * time.Second,
	}

	engine, err := NewHTTPTestEngine(cfg)
	if err != nil {
		t.Fatalf("new http test engine: %v", err)
	}
	defer engine.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pong":true}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	samples, err := measureHTTPPing(ctx, server.URL, 3)
	if err != nil {
		t.Fatalf("measureHTTPPing: %v", err)
	}
	if len(samples) != 3 {
		t.Fatalf("samples len = %d, want 3", len(samples))
	}
}

func TestMeasureHTTPPingHonorsContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatalf("measureHTTPPing: %v", err)
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
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"busy"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	samples, err := measureHTTPPing(ctx, server.URL, 3)
	if err != nil {
		t.Fatalf("measureHTTPPing: %v", err)
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
		t.Fatalf("measureHTTPPing: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("samples len = %d, want 0 when all requests fail", len(samples))
	}
}
