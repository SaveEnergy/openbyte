package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRateLimiterGlobalRefillLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 30
	rl := NewRateLimiter(cfg)
	rl.globalTokens = 0
	rl.globalLastRefill = time.Now().Add(-2 * time.Second)

	if !rl.allowGlobal() {
		t.Fatalf("expected global refill to allow request at low rate")
	}
}

func TestRateLimiterGlobalRefillVeryLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 10
	rl := NewRateLimiter(cfg)
	rl.globalTokens = 0
	rl.globalLastRefill = time.Now().Add(-6 * time.Second)

	if !rl.allowGlobal() {
		t.Fatalf("expected global refill to allow request at very low rate")
	}
}
