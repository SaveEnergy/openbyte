package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestResolveRandomSourceFallback(t *testing.T) {
	handler := NewSpeedTestHandler(2, 60)
	handler.randomData = nil

	src, release, err := handler.resolveRandomSource()
	if err != nil {
		t.Fatalf("resolveRandomSource: %v", err)
	}
	defer release()

	if len(src) != 64*1024 {
		t.Errorf("len(src) = %d, want %d", len(src), 64*1024)
	}
	if bytes.Equal(src, make([]byte, 64*1024)) {
		t.Error("expected non-zero random data")
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

	router := NewRouter(cfg, "test", nil)
	if got := router.speedtest.maxDurationSec; got != 1 {
		t.Fatalf("max duration seconds = %d, want safe 1-second fallback", got)
	}
}
