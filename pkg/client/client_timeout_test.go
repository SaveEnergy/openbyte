package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestSpeedTestLongDuration(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient.Timeout != 0 {
		t.Fatalf("default HTTP timeout = %v, want 0 (unbounded, context-driven)", c.httpClient.Timeout)
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
