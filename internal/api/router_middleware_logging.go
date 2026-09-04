package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const uploadRequestLogMinDuration = time.Second

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
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
				slog.Info("HTTP request",
					"method", req.Method,
					"path", path,
					"status", rw.statusCode,
					"duration_ms", float64(duration.Microseconds())/1000,
					"ip", r.resolveClientIP(req),
				)
			}
		} else {
			next.ServeHTTP(w, req)
		}
	})
}
