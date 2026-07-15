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

func TestNewClientHasNoGlobalHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != 0 {
		t.Fatalf("default http client timeout = %v, want no global timeout", c.httpClient.Timeout)
	}
}

func TestSpeedTestLongDurationUsesOperationDeadlines(t *testing.T) {
	var healthDeadline time.Time
	var latencyDeadlines []time.Time
	var downloadDeadline time.Time

	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Fatalf("%s request has no context deadline", req.URL.Path)
		}
		switch req.URL.Path {
		case pathHealth:
			healthDeadline = deadline
		case pathPing:
			latencyDeadlines = append(latencyDeadlines, deadline)
		case pathDownload:
			downloadDeadline = deadline
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}

	startedAt := time.Now()
	c := New("http://example.test", WithHTTPClient(httpClient))
	if _, err := c.SpeedTest(context.Background(), SpeedTestOptions{Duration: 120}); err != nil {
		t.Fatalf("SpeedTest: %v", err)
	}

	assertDeadlineWithin(t, "health", healthDeadline, healthRequestTimeout)
	if len(latencyDeadlines) != 5 {
		t.Fatalf("latency requests = %d, want 5", len(latencyDeadlines))
	}
	for _, deadline := range latencyDeadlines {
		if !deadline.Equal(latencyDeadlines[0]) {
			t.Fatalf("latency deadline = %v, want shared phase deadline %v", deadline, latencyDeadlines[0])
		}
		assertDeadlineWithin(t, "latency", deadline, latencyMeasurementTimeout)
	}
	if allowed := downloadDeadline.Sub(startedAt); allowed < 120*time.Second {
		t.Fatalf("download deadline = %v after start, want at least 2m", allowed)
	}
}

func TestPreflightDeadlinesHonorCallerContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	wantDeadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("caller context has no deadline")
	}

	var deadlines []time.Time
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		deadline, hasDeadline := req.Context().Deadline()
		if !hasDeadline {
			t.Fatalf("%s request has no context deadline", req.URL.Path)
		}
		deadlines = append(deadlines, deadline)
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
		}, nil
	})}
	c := New("http://example.test", WithHTTPClient(httpClient))
	if err := c.healthCheck(ctx); err != nil {
		t.Fatalf("healthCheck: %v", err)
	}
	if _, _, latencyOK := c.measureLatency(ctx, 2); !latencyOK {
		t.Fatal("measureLatency failed")
	}

	if len(deadlines) != 3 {
		t.Fatalf("preflight requests = %d, want 3", len(deadlines))
	}
	for _, deadline := range deadlines {
		if !deadline.Equal(wantDeadline) {
			t.Fatalf("preflight deadline = %v, want caller deadline %v", deadline, wantDeadline)
		}
	}
}

func assertDeadlineWithin(t *testing.T, operation string, deadline time.Time, limit time.Duration) {
	t.Helper()
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > limit {
		t.Fatalf("%s deadline remaining = %v, want > 0 and <= %v", operation, remaining, limit)
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
