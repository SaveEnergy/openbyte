package api

import (
	"net/http"
	"strings"
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
