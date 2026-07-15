package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

type clientRoundTripFunc func(*http.Request) (*http.Response, error)

func (f clientRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSpeedTestLongDurationUsesOperationDeadlines(t *testing.T) {
	var healthDeadline time.Time
	var latencyDeadlines []time.Time
	var downloadDeadline time.Time

	c := New("http://example.test")
	c.httpClient.Transport = clientRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			Request:    req,
		}, nil
	})

	startedAt := time.Now()
	if _, err := c.SpeedTest(context.Background(), SpeedTestOptions{Duration: 120}); err != nil {
		t.Fatalf("SpeedTest: %v", err)
	}
	assertDeadlineWithin(t, "health", healthDeadline, healthRequestTimeout)
	if len(latencyDeadlines) != 5 {
		t.Fatalf("latency requests = %d, want 5", len(latencyDeadlines))
	}
	for _, deadline := range latencyDeadlines {
		assertDeadlineWithin(t, "latency", deadline, latencyMeasurementTimeout)
	}
	if allowed := downloadDeadline.Sub(startedAt); allowed < 2*time.Minute {
		t.Fatalf("download deadline = %v after start, want at least 2m", allowed)
	}
}

func assertDeadlineWithin(t *testing.T, operation string, deadline time.Time, limit time.Duration) {
	t.Helper()
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > limit {
		t.Fatalf("%s deadline remaining = %v, want > 0 and <= %v", operation, remaining, limit)
	}
}

func TestInterpretationDoesNotAssumeZeroPacketLossWhenUnmeasured(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", handler.Download)
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)
	server := httptest.NewServer(mux)
	defer server.Close()

	c := New(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	for _, useCase := range result.Interpretation.SuitableFor {
		if useCase == "gaming" {
			t.Fatalf("gaming suitability should not be inferred when packet loss is unmeasured")
		}
	}
}
