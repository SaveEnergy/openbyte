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

const (
	jsonContentType    = "application/json"
	statusOKBody       = `{"status":"ok"}`
	unreachableBaseURL = "http://127.0.0.1:1"
	healthPath         = "GET /health"
	pingPath           = "GET /api/v1/ping"
	downloadPath       = "GET /api/v1/download"
	uploadPath         = "POST /api/v1/upload"
	downloadDirection  = "download"
	uploadDirection    = "upload"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentType)
		w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, handler.Ping)
	mux.HandleFunc(downloadPath, handler.Download)
	mux.HandleFunc(uploadPath, handler.Upload)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- Healthy ---

func TestSDKHealthyOK(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	if err := c.Healthy(context.Background()); err != nil {
		t.Fatalf("Healthy failed: %v", err)
	}
}

func TestSDKHealthyUnreachable(t *testing.T) {
	c := pkgclient.New(unreachableBaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c.Healthy(ctx) == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestSDKHealthyUnhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := pkgclient.New(srv.URL)
	if c.Healthy(context.Background()) == nil {
		t.Error("expected error for unhealthy server")
	}
}

// --- SpeedTest ---

func TestSDKSpeedTestDownload(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{
		Direction: downloadDirection,
		Duration:  1,
	})
	if err != nil {
		t.Fatalf("SpeedTest download failed: %v", err)
	}

	if result.Direction != downloadDirection {
		t.Errorf("expected direction=%s, got %s", downloadDirection, result.Direction)
	}
	if result.ThroughputMbps <= 0 {
		t.Error("expected throughput > 0")
	}
	if result.Interpretation == nil {
		t.Fatal("interpretation should not be nil")
	}
}

func TestSDKSpeedTestUpload(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{
		Direction: uploadDirection,
		Duration:  1,
	})
	if err != nil {
		t.Fatalf("SpeedTest upload failed: %v", err)
	}

	if result.Direction != uploadDirection {
		t.Errorf("expected direction=%s, got %s", uploadDirection, result.Direction)
	}
	if result.BytesTotal <= 0 {
		t.Error("expected bytes_total > 0")
	}
}

func TestSDKSpeedTestDefaultDirection(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 1})
	if err != nil {
		t.Fatalf("SpeedTest failed: %v", err)
	}
	if result.Direction != downloadDirection {
		t.Errorf("expected default direction=%s, got %s", downloadDirection, result.Direction)
	}
}

func TestSDKSpeedTestInvalidDirection(t *testing.T) {
	srv := newTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Direction: "bidirectional"})
	if err == nil {
		t.Error("expected error for invalid direction")
	}
}

func TestSDKSpeedTestDurationClamped(t *testing.T) {
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

func TestSDKSpeedTestUnreachableServer(t *testing.T) {
	c := pkgclient.New(unreachableBaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 1})
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- WithAPIKey ---

func TestSDKWithAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentType)
		w.Write([]byte(statusOKBody))
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

func TestSDKCheckHasInterpretation(t *testing.T) {
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

func TestSDKCheckReturnsLatencyMeasurementErrorWhenPingFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentType)
		_, _ = w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc(downloadPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
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

func TestSDKSpeedTestReturnsDownloadMeasurementErrorWhenDownloadFails(t *testing.T) {
	mux := http.NewServeMux()
	handler := api.NewSpeedTestHandler(10, 300)
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentType)
		_, _ = w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, handler.Ping)
	mux.HandleFunc(downloadPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc(uploadPath, handler.Upload)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: downloadDirection,
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrDownloadMeasurementFailed) {
		t.Fatalf("err = %v, want ErrDownloadMeasurementFailed", err)
	}
}

func TestSDKSpeedTestDownloadUnexpectedEOF(t *testing.T) {
	mux := http.NewServeMux()
	handler := api.NewSpeedTestHandler(10, 300)
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", jsonContentType)
		_, _ = w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, handler.Ping)
	mux.HandleFunc(downloadPath, func(w http.ResponseWriter, r *http.Request) {
		// Force body truncation so client sees non-EOF read error.
		w.Header().Set("Content-Length", "1048576")
		_, _ = fmt.Fprint(w, "short")
	})
	mux.HandleFunc(uploadPath, handler.Upload)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: downloadDirection,
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrDownloadMeasurementFailed) {
		t.Fatalf("err = %v, want ErrDownloadMeasurementFailed", err)
	}
}

func TestSDKSpeedTestUploadDurationImpactsWorkload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := pkgclient.New(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res1, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Direction: uploadDirection, Duration: 1})
	if err != nil {
		t.Fatalf("duration=1 speed test failed: %v", err)
	}
	bytes1 := res1.BytesTotal

	ctx2, cancel2 := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel2()
	res2, err := c.SpeedTest(ctx2, pkgclient.SpeedTestOptions{Direction: uploadDirection, Duration: 2})
	if err != nil {
		t.Fatalf("duration=2 speed test failed: %v", err)
	}
	bytes2 := res2.BytesTotal

	if bytes2 <= bytes1 {
		t.Fatalf("bytes for longer duration should increase: duration1=%d duration2=%d", bytes1, bytes2)
	}
}

func TestSDKMeasureLatencyMinimumSamplesForJitter(t *testing.T) {
	var pingCount int
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(statusOKBody))
	})
	mux.HandleFunc(pingPath, func(w http.ResponseWriter, r *http.Request) {
		pingCount++
		if pingCount == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(downloadPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := pkgclient.New(srv.URL)
	_, err := c.SpeedTest(context.Background(), pkgclient.SpeedTestOptions{
		Direction: uploadDirection,
		Duration:  1,
	})
	if !errors.Is(err, pkgclient.ErrLatencyMeasurementFailed) {
		t.Fatalf("err = %v, want ErrLatencyMeasurementFailed", err)
	}
}
