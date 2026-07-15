package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testPathAPIUpload = "/api/v1/upload"

type deadlineResponseWriter struct {
	header        http.Header
	readDeadline  time.Time
	writeDeadline time.Time
}

func (w *deadlineResponseWriter) Header() http.Header {
	return w.header
}

func (w *deadlineResponseWriter) Write(body []byte) (int, error) {
	return len(body), nil
}

func (w *deadlineResponseWriter) WriteHeader(int) {}

func (w *deadlineResponseWriter) SetReadDeadline(deadline time.Time) error {
	w.readDeadline = deadline
	return nil
}

func (w *deadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	return nil
}

func TestLoggingMiddlewarePreservesResponseControllerDeadlines(t *testing.T) {
	readDeadline := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	writeDeadline := readDeadline.Add(time.Second)
	underlying := &deadlineResponseWriter{header: make(http.Header)}

	handler := (&Router{}).LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		controller := http.NewResponseController(w)
		if err := controller.SetReadDeadline(readDeadline); err != nil {
			t.Fatalf("SetReadDeadline: %v", err)
		}
		if err := controller.SetWriteDeadline(writeDeadline); err != nil {
			t.Fatalf("SetWriteDeadline: %v", err)
		}
	}))
	handler.ServeHTTP(underlying, httptest.NewRequest(http.MethodGet, "/api/v1/download", nil))

	if !underlying.readDeadline.Equal(readDeadline) {
		t.Fatalf("read deadline = %v, want %v", underlying.readDeadline, readDeadline)
	}
	if !underlying.writeDeadline.Equal(writeDeadline) {
		t.Fatalf("write deadline = %v, want %v", underlying.writeDeadline, writeDeadline)
	}
}

func TestShouldSkipRequestLog(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/api/v1/ping", want: true},
		{path: "/ping", want: true},
		{path: "/api/v1/download", want: false},
		{path: testPathAPIUpload, want: false},
		{path: "/api/v1/results/abc12345", want: false},
		{path: "/api/v1/ping/extra", want: false},
		{path: "ping", want: false},
		{path: "", want: false},
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
		{name: "upload success stays quiet", path: testPathAPIUpload, status: http.StatusOK, duration: 10 * time.Millisecond, want: false},
		{name: "upload failure logs", path: testPathAPIUpload, status: http.StatusServiceUnavailable, duration: 10 * time.Millisecond, want: true},
		{name: "slow upload logs", path: testPathAPIUpload, status: http.StatusOK, duration: uploadRequestLogMinDuration + time.Millisecond, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldLogRequest(tt.path, tt.status, tt.duration); got != tt.want {
				t.Fatalf("shouldLogRequest(%q, %d, %s) = %t, want %t", tt.path, tt.status, tt.duration, got, tt.want)
			}
		})
	}
}
