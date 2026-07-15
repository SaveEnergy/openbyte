package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestNewSpeedTestHandlerNormalizesImmutablePolicy(t *testing.T) {
	handler := NewSpeedTestHandlerWithPolicy(2, 60, -1, nil)

	if handler.maxConcurrentPerIP != 0 {
		t.Fatalf("max concurrent per IP = %d, want 0", handler.maxConcurrentPerIP)
	}
	if handler.clientIPResolver == nil {
		t.Fatal("expected default client IP resolver")
	}
}

func TestUploadReadDeadline(t *testing.T) {
	start := time.Date(2026, 2, 15, 7, 0, 0, 0, time.UTC)

	if got := uploadReadDeadline(start, 60); !got.Equal(start.Add(60 * time.Second)) {
		t.Fatalf("deadline = %v, want %v", got, start.Add(60*time.Second))
	}

	if got := uploadReadDeadline(start, 0); !got.Equal(start.Add(300 * time.Second)) {
		t.Fatalf("default deadline = %v, want %v", got, start.Add(300*time.Second))
	}
}

func TestUploadReadBufferSizedForHighThroughput(t *testing.T) {
	if uploadReadBufferSize != 1024*1024 {
		t.Fatalf("uploadReadBufferSize = %d, want 1048576", uploadReadBufferSize)
	}
}

func TestParseDownloadParamsBoundsDefaultByConfiguredMaximum(t *testing.T) {
	tests := []struct {
		maxDurationSec int
		wantDuration   time.Duration
	}{
		{maxDurationSec: 1, wantDuration: time.Second},
		{maxDurationSec: 5, wantDuration: 5 * time.Second},
		{maxDurationSec: 10, wantDuration: 10 * time.Second},
		{maxDurationSec: 300, wantDuration: 10 * time.Second},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/download", nil)
		duration, _, err := parseDownloadParams(req, tt.maxDurationSec)
		if err != nil {
			t.Fatalf("max %d: parseDownloadParams: %v", tt.maxDurationSec, err)
		}
		if duration != tt.wantDuration {
			t.Fatalf("max %d: duration = %v, want %v", tt.maxDurationSec, duration, tt.wantDuration)
		}
	}
}

func TestNewRouterDoesNotBroadenFractionalDurationLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 500 * time.Millisecond

	router := NewRouter(cfg, nil)
	if got := router.speedtest.maxDurationSec; got != 1 {
		t.Fatalf("max duration seconds = %d, want safe 1-second fallback", got)
	}
}

func TestNewRouterUsesExplicitTransferLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentTransfers = 73

	router := NewRouter(cfg, nil)
	if got := router.speedtest.maxConcurrent; got != 73 {
		t.Fatalf("max concurrent transfers = %d, want 73", got)
	}
}
