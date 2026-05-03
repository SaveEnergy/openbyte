package api

import "net/http"

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
