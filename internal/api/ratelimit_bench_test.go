package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

// BenchmarkRateLimiterAllow is the per-request gate before non-speedtest handlers (global + per-IP buckets).
func BenchmarkRateLimiterAllow(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 50_000_000
	cfg.RateLimitPerIP = 50_000_000
	rl := NewRateLimiter(cfg)
	rl.SetCleanupPolicy(24*time.Hour, 24*time.Hour)
	ip := "198.51.100.77"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !rl.Allow(ip) {
			b.Fatal("unexpected deny")
		}
	}
}
