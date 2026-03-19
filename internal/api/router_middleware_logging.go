package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

const uploadRequestLogMinDuration = time.Second

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func shouldSkipRequestLog(path string) bool {
	return strings.HasSuffix(path, "/ping")
}

func shouldLogRequest(path string, status int, duration time.Duration) bool {
	if !strings.HasPrefix(path, "/api/") || shouldSkipRequestLog(path) {
		return false
	}
	if strings.HasSuffix(path, "/upload") {
		return status >= http.StatusBadRequest || duration >= uploadRequestLogMinDuration
	}
	return true
}

func (r *Router) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		if strings.HasPrefix(path, "/api/") && !shouldSkipRequestLog(path) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, req)

			duration := time.Since(start)
			if shouldLogRequest(path, rw.statusCode, duration) {
				logging.Info("HTTP request",
					logging.Field{Key: "method", Value: req.Method},
					logging.Field{Key: "path", Value: path},
					logging.Field{Key: "status", Value: rw.statusCode},
					logging.Field{Key: "duration_ms", Value: float64(duration.Microseconds()) / 1000},
					logging.Field{Key: "ip", Value: r.resolveClientIP(req)},
				)
			}
		} else {
			next.ServeHTTP(w, req)
		}
	})
}
