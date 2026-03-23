package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func benchRateLimiterHigh() *RateLimiter {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 50_000_000
	cfg.RateLimitPerIP = 50_000_000
	lim := NewRateLimiter(cfg)
	lim.SetCleanupPolicy(24*time.Hour, 24*time.Hour)
	return lim
}

// BenchmarkRegistryRateLimitMiddlewareBypass is the common path (non-registry API routes).
func BenchmarkRegistryRateLimitMiddlewareBypass(b *testing.B) {
	lim := benchRateLimiterHigh()
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h := registryRateLimitMiddleware(lim, next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/download", nil)
	req.RemoteAddr = "192.0.2.1:1"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// BenchmarkRegistryRateLimitMiddlewareRegistryHit exercises /api/v1/registry/* + Allow.
func BenchmarkRegistryRateLimitMiddlewareRegistryHit(b *testing.B) {
	lim := benchRateLimiterHigh()
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h := registryRateLimitMiddleware(lim, next)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/servers", nil)
	req.RemoteAddr = "192.0.2.1:1"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// BenchmarkStaticCacheMiddleware sets cache headers then delegates (GET HTML path).
func BenchmarkStaticCacheMiddleware(b *testing.B) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h := staticCacheMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkIsHTTPSPlain(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if isHTTPS(req) {
			b.Fatal("expected http")
		}
	}
}

func BenchmarkIsHTTPSForwarded(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isHTTPS(req) {
			b.Fatal("expected https")
		}
	}
}
