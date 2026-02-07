package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	pkgclient "github.com/saveenergy/openbyte/pkg/client"
)

// TestCheckQuick_HealthyServer verifies Check returns results against a real httptest server.
func TestCheckQuick_HealthyServer(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", handler.Download)
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := pkgclient.New(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.Status != "ok" {
		t.Errorf("expected status ok, got %s", result.Status)
	}
	if result.LatencyMs <= 0 {
		t.Error("expected latency > 0")
	}
	if result.Interpretation == nil {
		t.Fatal("interpretation should not be nil")
	}
	if result.Interpretation.Grade == "" {
		t.Error("grade should not be empty")
	}
	if result.DurationMs <= 0 {
		t.Error("duration_ms should be > 0")
	}
}

// TestCheckQuick_UnreachableServer verifies Check returns error for bad server.
func TestCheckQuick_UnreachableServer(t *testing.T) {
	c := pkgclient.New("http://127.0.0.1:1") // port 1 — unreachable
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.Check(ctx)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// TestCheckQuick_UnhealthyServer verifies Check returns error when health returns non-200.
func TestCheckQuick_UnhealthyServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	c := pkgclient.New(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Check(ctx)
	if err == nil {
		t.Fatal("expected error for unhealthy server")
	}
}

// TestCheckQuick_JSONSerializable verifies the result can be marshaled to JSON.
func TestCheckQuick_JSONSerializable(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", handler.Download)
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c := pkgclient.New(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify key fields exist
	for _, key := range []string{"status", "server_url", "latency_ms", "interpretation"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expected field %q in JSON output", key)
		}
	}
}
