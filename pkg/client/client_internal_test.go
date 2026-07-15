package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientHasDefaultHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("default http client timeout = %v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
	}
}

func TestUploadMeasuredAllowsFirstResponsePastTargetDuration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathUpload, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(25 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL)
	mbps, totalBytes, ok := c.uploadMeasured(context.Background(), 0)
	if !ok {
		t.Fatalf("uploadMeasured ok = false, mbps=%f totalBytes=%d", mbps, totalBytes)
	}
	if totalBytes == 0 {
		t.Fatal("expected uploadMeasured to count the completed upload")
	}
}

func TestNormalizeSpeedTestOptions(t *testing.T) {
	tests := []struct {
		name      string
		input     SpeedTestOptions
		want      SpeedTestOptions
		wantError bool
	}{
		{name: "defaults", input: SpeedTestOptions{}, want: SpeedTestOptions{Direction: directionDownload, Duration: 1}},
		{name: "upper duration bound", input: SpeedTestOptions{Direction: directionUpload, Duration: 301}, want: SpeedTestOptions{Direction: directionUpload, Duration: 300}},
		{name: "invalid direction", input: SpeedTestOptions{Direction: "bidirectional", Duration: 1}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeSpeedTestOptions(tt.input)
			if (err != nil) != tt.wantError {
				t.Fatalf("normalizeSpeedTestOptions() error = %v, wantError = %v", err, tt.wantError)
			}
			if got != tt.want {
				t.Fatalf("normalizeSpeedTestOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
