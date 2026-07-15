package api

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRateLimiterGlobalRefillLowRates(t *testing.T) {
	tests := []struct {
		name    string
		rate    int
		elapsed time.Duration
	}{
		{name: "thirty per minute", rate: 30, elapsed: 2 * time.Second},
		{name: "ten per minute", rate: 10, elapsed: 6 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.GlobalRateLimit = test.rate
			limiter := NewRateLimiter(cfg)
			start := time.Unix(1, 0)
			limiter.globalTokens = 0
			limiter.globalLastRefill = start

			limiter.refillGlobalTokens(start.Add(test.elapsed))

			if limiter.globalTokens != 1 {
				t.Fatalf("tokens = %d, want 1", limiter.globalTokens)
			}
		})
	}
}

func TestRateLimiterIPRefillLowRates(t *testing.T) {
	tests := []struct {
		name    string
		rate    int
		elapsed time.Duration
	}{
		{name: "thirty per minute", rate: 30, elapsed: 2 * time.Second},
		{name: "ten per minute", rate: 10, elapsed: 6 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.RateLimitPerIP = test.rate
			limiter := NewRateLimiter(cfg)
			start := time.Unix(1, 0)
			limit := &IPLimit{lastRefill: start}

			limiter.refillIPTokens(limit, start.Add(test.elapsed))

			if limit.tokens != 1 {
				t.Fatalf("tokens = %d, want 1", limit.tokens)
			}
		})
	}
}

func TestCleanupExpiredIPLimits(t *testing.T) {
	cfg := config.DefaultConfig()
	rl := NewRateLimiter(cfg)
	now := time.Unix(100, 0)
	rl.ipLimitTTL = time.Minute
	rl.ipLimits["expired"] = &IPLimit{tokens: 1, lastRefill: now.Add(-2 * time.Minute)}
	rl.ipLimits["current"] = &IPLimit{tokens: 1, lastRefill: now}

	rl.cleanupExpiredIPLimits(now)

	if _, ok := rl.ipLimits["expired"]; ok {
		t.Fatal("expired IP limit was not removed")
	}
	if _, ok := rl.ipLimits["current"]; !ok {
		t.Fatal("current IP limit was removed")
	}
}

func TestRateLimiterCleanupConcurrentAllowStress(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 5000
	cfg.RateLimitPerIP = 5000
	limiter := NewRateLimiter(cfg)
	limiter.cleanupInterval = 2 * time.Millisecond
	limiter.ipLimitTTL = 4 * time.Millisecond
	limiter.lastCleanup = time.Now().Add(-limiter.cleanupInterval)

	var wg sync.WaitGroup
	for worker := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range 2_000 {
				limiter.Allow(fmt.Sprintf("10.0.%d.%d", worker, n%64))
			}
		}()
	}
	wg.Wait()
}

func TestRateLimiterBoundedIPCardinality(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 1000
	limiter := NewRateLimiter(cfg)
	limiter.maxIPEntries = 2

	if !limiter.Allow("10.0.0.1") {
		t.Fatal("first IP should be allowed")
	}
	if !limiter.Allow("10.0.0.2") {
		t.Fatal("second IP should be allowed")
	}
	if limiter.Allow("10.0.0.3") {
		t.Fatal("third unique IP should be rejected once map is full")
	}
}
