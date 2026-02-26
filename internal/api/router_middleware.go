package api

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

func registryRateLimitMiddleware(limiter *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, apiV1RegistryPrefix) {
			next.ServeHTTP(w, r)
			return
		}
		if skipRateLimitPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		ip := limiter.ClientIP(r)
		if !limiter.Allow(ip) {
			w.Header().Set(headerRetryAfter, retryAfterSec)
			respondJSON(w, map[string]string{"error": errRateLimitExceeded}, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// applyRateLimit wraps a handler with rate limit checking.
func applyRateLimit(limiter *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if skipRateLimitPaths[r.URL.Path] {
			next(w, r)
			return
		}
		ip := limiter.ClientIP(r)
		if !limiter.Allow(ip) {
			w.Header().Set(headerRetryAfter, retryAfterSec)
			respondJSON(w, map[string]string{"error": errRateLimitExceeded}, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (r *Router) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		originAllowed := origin != "" && r.isAllowedOrigin(origin)
		if originAllowed {
			allowOrigin := origin
			if r.isAllowAllOrigins() {
				allowOrigin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			if allowOrigin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}
		if req.Method == http.MethodOptions {
			if origin != "" && !originAllowed {
				respondJSON(w, map[string]string{"error": errOriginNotAllowed}, http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (r *Router) isAllowedOrigin(origin string) bool {
	if len(r.allowedOrigins) == 0 {
		return false
	}
	originHostValue := types.OriginHost(origin)
	for _, allowed := range r.allowedOrigins {
		if matchesAllowedOrigin(strings.TrimSpace(allowed), origin, originHostValue) {
			return true
		}
	}
	return false
}

func matchesAllowedOrigin(allowed, origin, originHostValue string) bool {
	if allowed == "" {
		return false
	}
	if allowed == "*" || strings.EqualFold(allowed, origin) {
		return true
	}
	if after, ok := strings.CutPrefix(allowed, "*."); ok {
		return originHostValue != "" &&
			(originHostValue == after || strings.HasSuffix(originHostValue, "."+after))
	}
	allowedHost := types.OriginHost(allowed)
	return allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue)
}

func (r *Router) isAllowAllOrigins() bool {
	return slices.Contains(r.allowedOrigins, "*")
}

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

func (r *Router) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		skipLog := strings.HasSuffix(path, "/stream") ||
			strings.HasSuffix(path, "/download") ||
			strings.HasSuffix(path, "/upload") ||
			strings.HasSuffix(path, "/ping")

		if strings.HasPrefix(path, "/api/") && !skipLog {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, req)

			duration := time.Since(start)
			logging.Info("HTTP request",
				logging.Field{Key: "method", Value: req.Method},
				logging.Field{Key: "path", Value: path},
				logging.Field{Key: "status", Value: rw.statusCode},
				logging.Field{Key: "duration_ms", Value: float64(duration.Microseconds()) / 1000},
				logging.Field{Key: "ip", Value: r.resolveClientIP(req)},
			)
		} else {
			next.ServeHTTP(w, req)
		}
	})
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"font-src 'self'; "+
				"style-src 'self'; "+
				"script-src 'self'; "+
				"img-src 'self' data:; "+
				"connect-src 'self' https: http: ws: wss:")
		next.ServeHTTP(w, r)
	})
}

func DeadlineMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deadline, ok := r.Context().Deadline(); ok {
			controller := http.NewResponseController(w)
			_ = controller.SetWriteDeadline(deadline.Add(5 * time.Second))
		}
		next.ServeHTTP(w, r)
	})
}
