package api

import (
	"log/slog"
	"net/http"
	"time"
)

const uploadRequestLogMinDuration = time.Second

const (
	apiPathPrefix       = "/api/"
	apiPathPrefixLen    = len(apiPathPrefix)
	uploadPathSuffix    = "/upload"
	uploadPathSuffixLen = len(uploadPathSuffix)
)

func hasAPIPathPrefix(path string) bool {
	return len(path) >= apiPathPrefixLen && path[:apiPathPrefixLen] == apiPathPrefix
}

func hasUploadPathSuffix(path string) bool {
	lp := len(path)
	return lp >= uploadPathSuffixLen && path[lp-uploadPathSuffixLen:] == uploadPathSuffix
}

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
	const suf = "/ping"
	lp := len(path)
	ls := len(suf)
	return lp >= ls && path[lp-ls:] == suf
}

func shouldLogRequest(path string, status int, duration time.Duration) bool {
	if !hasAPIPathPrefix(path) || shouldSkipRequestLog(path) {
		return false
	}
	if hasUploadPathSuffix(path) {
		return status >= http.StatusBadRequest || duration >= uploadRequestLogMinDuration
	}
	return true
}

func (r *Router) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		if hasAPIPathPrefix(path) && !shouldSkipRequestLog(path) {
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
