package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestHTTPTestEngineDownload(t *testing.T) {
	handler := api.NewSpeedTestHandler(10)
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

	engine := NewHTTPTestEngine(cfg)
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
	handler := api.NewSpeedTestHandler(10)
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

	engine := NewHTTPTestEngine(cfg)
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
