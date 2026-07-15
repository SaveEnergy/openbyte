package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRefillGlobalTokensAtLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 10
	rl := NewRateLimiter(cfg)
	start := time.Unix(1, 0)
	rl.globalTokens = 0
	rl.globalLastRefill = start

	rl.refillGlobalTokens(start.Add(6 * time.Second))

	if rl.globalTokens != 1 {
		t.Fatalf("global tokens = %d, want 1", rl.globalTokens)
	}
}

func TestRefillIPTokensAtLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RateLimitPerIP = 30
	rl := NewRateLimiter(cfg)
	start := time.Unix(1, 0)
	limit := &IPLimit{lastRefill: start}

	rl.refillIPTokens(limit, start.Add(2*time.Second))

	if limit.tokens != 1 {
		t.Fatalf("IP tokens = %d, want 1", limit.tokens)
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
