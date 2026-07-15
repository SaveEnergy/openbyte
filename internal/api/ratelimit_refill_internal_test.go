package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRateLimiterGlobalRefillLowRates(t *testing.T) {
	for _, test := range []struct {
		rate    int
		elapsed time.Duration
	}{
		{rate: 30, elapsed: 2 * time.Second},
		{rate: 10, elapsed: 6 * time.Second},
	} {
		cfg := config.DefaultConfig()
		cfg.GlobalRateLimit = test.rate
		limiter := NewRateLimiter(cfg)
		limiter.globalTokens = 0
		start := limiter.globalLastRefill

		limiter.refillGlobalTokens(start.Add(test.elapsed))

		if limiter.globalTokens != 1 {
			t.Errorf("rate %d: tokens = %d, want 1", test.rate, limiter.globalTokens)
		}
	}
}

func TestRateLimiterIPRefillLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RateLimitPerIP = 30
	limiter := NewRateLimiter(cfg)
	start := time.Now()
	ipLimit := &IPLimit{lastRefill: start}

	limiter.refillIPTokens(ipLimit, start.Add(2*time.Second))

	if ipLimit.tokens != 1 {
		t.Fatalf("tokens = %d, want 1", ipLimit.tokens)
	}
}
