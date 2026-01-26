package api_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

func drainGlobalTokens(t *testing.T, rl *api.RateLimiter, ip string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("token %d not allowed", i)
		}
	}
}

func TestRateLimiterGlobalRefillLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 30
	cfg.RateLimitPerIP = 1000
	rl := api.NewRateLimiter(cfg)

	ip := "127.0.0.1"
	drainGlobalTokens(t, rl, ip, cfg.GlobalRateLimit)

	time.Sleep(2 * time.Second)

	if !rl.Allow(ip) {
		t.Fatalf("expected global refill to allow request at low rate")
	}
}

func TestRateLimiterGlobalRefillVeryLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 10
	cfg.RateLimitPerIP = 1000
	rl := api.NewRateLimiter(cfg)

	ip := "127.0.0.1"
	drainGlobalTokens(t, rl, ip, cfg.GlobalRateLimit)

	time.Sleep(6 * time.Second)

	if !rl.Allow(ip) {
		t.Fatalf("expected global refill to allow request at very low rate")
	}
}
