package client_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	pkgclient "github.com/saveenergy/openbyte/pkg/client"
)

const (
	headerContentType  = "Content-Type"
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
		w.Header().Set(headerContentType, jsonContentType)
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

func TestSDKSpeedTestUnreachableServer(t *testing.T) {
	c := pkgclient.New(unreachableBaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{Duration: 1})
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// --- Check ---

func TestSDKCheckReturnsLatencyMeasurementErrorWhenPingFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, jsonContentType)
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
		w.Header().Set(headerContentType, jsonContentType)
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
		w.Header().Set(headerContentType, jsonContentType)
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
