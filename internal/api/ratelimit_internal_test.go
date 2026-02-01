package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRateLimiterCleanupRemovesStaleEntries(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 1000

	rl := NewRateLimiter(cfg)
	rl.SetCleanupPolicy(10*time.Millisecond, 20*time.Millisecond)

	ip := "127.0.0.1"
	if !rl.Allow(ip) {
		t.Fatalf("expected allow on first request")
	}

	rl.ipMu.Lock()
	rl.ipLimits[ip].lastRefill = time.Now().Add(-time.Minute)
	rl.lastCleanup = time.Now().Add(-time.Minute)
	rl.ipMu.Unlock()

	rl.Allow("127.0.0.2")

	rl.ipMu.RLock()
	_, exists := rl.ipLimits[ip]
	rl.ipMu.RUnlock()
	if exists {
		t.Fatalf("expected stale ip limit to be cleaned up")
	}
}
