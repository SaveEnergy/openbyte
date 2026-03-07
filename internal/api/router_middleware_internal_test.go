package api

import (
	"net/http"
	"testing"
	"time"
)

func TestShouldSkipRequestLog(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/v1/ping", want: true},
		{path: "/api/v1/download", want: false},
		{path: "/api/v1/upload", want: false},
		{path: "/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream", want: false},
		{path: "/api/v1/results/abc12345", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := shouldSkipRequestLog(tt.path); got != tt.want {
				t.Fatalf("shouldSkipRequestLog(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestShouldLogRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		status   int
		duration time.Duration
		want     bool
	}{
		{name: "ping stays skipped", path: "/api/v1/ping", status: http.StatusOK, duration: 10 * time.Millisecond, want: false},
		{name: "download logs", path: "/api/v1/download", status: http.StatusOK, duration: 10 * time.Millisecond, want: true},
		{name: "upload success stays quiet", path: "/api/v1/upload", status: http.StatusOK, duration: 10 * time.Millisecond, want: false},
		{name: "upload failure logs", path: "/api/v1/upload", status: http.StatusServiceUnavailable, duration: 10 * time.Millisecond, want: true},
		{name: "slow upload logs", path: "/api/v1/upload", status: http.StatusOK, duration: uploadRequestLogMinDuration + time.Millisecond, want: true},
		{name: "stream websocket logs", path: "/api/v1/stream/550e8400-e29b-41d4-a716-446655440000/stream", status: http.StatusSwitchingProtocols, duration: 5 * time.Second, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldLogRequest(tt.path, tt.status, tt.duration); got != tt.want {
				t.Fatalf("shouldLogRequest(%q, %d, %s) = %t, want %t", tt.path, tt.status, tt.duration, got, tt.want)
			}
		})
	}
}
