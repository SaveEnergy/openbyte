package client_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	pkgclient "github.com/saveenergy/openbyte/pkg/client"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", handler.Download)
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- Healthy ---

func TestSDK_Healthy_OK(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	if err := c.Healthy(context.Background()); err != nil {
		t.Fatalf("Healthy failed: %v", err)
	}
}

func TestSDK_Healthy_Unreachable(t *testing.T) {
	c := pkgclient.New("http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Healthy(ctx); err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestSDK_Healthy_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := pkgclient.New(srv.URL)
	if err := c.Healthy(context.Background()); err == nil {
		t.Error("expected error for unhealthy server")
	}
}

// --- SpeedTest ---

func TestSDK_SpeedTest_Download(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{
		Direction: "download",
		Duration:  1,
	})
	if err != nil {
		t.Fatalf("SpeedTest download failed: %v", err)
	}

	if result.Direction != "download" {
		t.Errorf("expected direction=download, got %s", result.Direction)
	}
	if result.ThroughputMbps <= 0 {
		t.Error("expected throughput > 0")
	}
	if result.Interpretation == nil {
		t.Fatal("interpretation should not be nil")
	}
}

func TestSDK_SpeedTest_Upload(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{
		Direction: "upload",
		Duration:  1,
	})
	if err != nil {
		t.Fatalf("SpeedTest upload failed: %v", err)
	}

	if result.Direction != "upload" {
		t.Errorf("expected direction=upload, got %s", result.Direction)
	}
	if result.BytesTotal <= 0 {
		t.Error("expected bytes_total > 0")
	}
}

func TestSDK_SpeedTest_DefaultDirection(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 1})
	if err != nil {
		t.Fatalf("SpeedTest failed: %v", err)
	}
	if result.Direction != "download" {
		t.Errorf("expected default direction=download, got %s", result.Direction)
	}
}

func TestSDK_SpeedTest_InvalidDirection(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Direction: "bidirectional"})
	if err == nil {
		t.Error("expected error for invalid direction")
	}
}

func TestSDK_SpeedTest_DurationClamped(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Duration 0 should be clamped to 1
	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 0})
	if err != nil {
		t.Fatalf("SpeedTest failed: %v", err)
	}
	if result.DurationSec < 0.5 {
		t.Errorf("expected some duration, got %.2f", result.DurationSec)
	}
}

func TestSDK_SpeedTest_UnreachableServer(t *testing.T) {
	c := pkgclient.New("http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 1})
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- WithAPIKey ---

func TestSDK_WithAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	c := pkgclient.New(srv.URL, pkgclient.WithAPIKey("test-key-123"))
	if err := c.Healthy(context.Background()); err != nil {
		t.Fatalf("Healthy failed: %v", err)
	}
	if c == nil {
		t.Error("client should not be nil")
	}
}

// --- Check ---

func TestSDK_Check_HasInterpretation(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if result.Interpretation == nil {
		t.Fatal("interpretation should not be nil")
	}
	if result.Interpretation.Grade == "" {
		t.Error("grade should not be empty")
	}
	if result.Interpretation.Summary == "" {
		t.Error("summary should not be empty")
	}
	if result.Interpretation.SuitableFor == nil {
		t.Error("suitable_for should not be nil")
	}
	if result.Interpretation.Concerns == nil {
		t.Error("concerns should not be nil")
	}
}

func TestSDK_Check_ReturnsLatencyMeasurementErrorWhenPingFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("GET /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := pkgclient.New(srv.URL)
	_, err := c.Check(context.Background())
	if !errors.Is(err, pkgclient.ErrLatencyMeasurementFailed) {
		t.Fatalf("err = %v, want ErrLatencyMeasurementFailed", err)
	}
}

func TestSDK_SpeedTest_ReturnsDownloadMeasurementErrorWhenDownloadFails(t *testing.T) {
	mux := http.NewServeMux()
	handler := api.NewSpeedTestHandler(10, 300)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: "download",
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrDownloadMeasurementFailed) {
		t.Fatalf("err = %v, want ErrDownloadMeasurementFailed", err)
	}
}

func TestSDK_SpeedTest_DownloadUnexpectedEOF(t *testing.T) {
	mux := http.NewServeMux()
	handler := api.NewSpeedTestHandler(10, 300)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		// Force body truncation so client sees non-EOF read error.
		w.Header().Set("Content-Length", "1048576")
		_, _ = fmt.Fprint(w, "short")
	})
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: "download",
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrDownloadMeasurementFailed) {
		t.Fatalf("err = %v, want ErrDownloadMeasurementFailed", err)
	}
}

func TestSDK_SpeedTest_UploadDurationImpactsWorkload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("POST /api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := pkgclient.New(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res1, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Direction: "upload", Duration: 1})
	if err != nil {
		t.Fatalf("duration=1 speed test failed: %v", err)
	}
	bytes1 := res1.BytesTotal

	ctx2, cancel2 := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel2()
	res2, err := c.SpeedTest(ctx2, pkgclient.SpeedTestOptions{Direction: "upload", Duration: 2})
	if err != nil {
		t.Fatalf("duration=2 speed test failed: %v", err)
	}
	bytes2 := res2.BytesTotal

	if bytes2 <= bytes1 {
		t.Fatalf("bytes for longer duration should increase: duration1=%d duration2=%d", bytes1, bytes2)
	}
}

func TestSDK_MeasureLatency_MinimumSamplesForJitter(t *testing.T) {
	var pingCount int
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		pingCount++
		if pingCount == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("POST /api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/v1/download", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: "upload",
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrLatencyMeasurementFailed) {
		t.Fatalf("err = %v, want ErrLatencyMeasurementFailed", err)
	}
}
