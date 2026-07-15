package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewClientHasDefaultHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("default http client timeout = %v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
	}
}

func TestUploadMeasuredUsesResponseGraceDeadline(t *testing.T) {
	var remaining time.Duration
	var hasDeadline bool
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		hasDeadline = ok
		if ok {
			remaining = time.Until(deadline)
		}
		// Take measurable time so elapsed > 0 even on platforms with a coarse
		// monotonic clock (Windows); an instant response makes ok=false.
		time.Sleep(5 * time.Millisecond)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})}

	c := New("http://example.test", WithHTTPClient(httpClient))
	mbps, totalBytes, ok := c.uploadMeasured(context.Background(), 0)
	if !ok {
		t.Fatalf("uploadMeasured ok = false, mbps=%f totalBytes=%d", mbps, totalBytes)
	}
	if totalBytes == 0 {
		t.Fatal("expected uploadMeasured to count the completed upload")
	}
	if !hasDeadline {
		t.Fatal("upload request context has no deadline")
	}
	if remaining < uploadMeasurementGrace-time.Second || remaining > uploadMeasurementGrace+time.Second {
		t.Fatalf("upload request deadline remaining = %v, want about %v", remaining, uploadMeasurementGrace)
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
