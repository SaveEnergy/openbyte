package api_test

import (
	"fmt"
	"sync"
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

func TestRateLimiterIPRefillLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 30
	rl := api.NewRateLimiter(cfg)

	ip := "127.0.0.1"
	for i := 0; i < cfg.RateLimitPerIP; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("token %d not allowed", i)
		}
	}

	time.Sleep(2 * time.Second)

	if !rl.Allow(ip) {
		t.Fatalf("expected per-ip refill to allow request at low rate")
	}
}

func TestRateLimiterIndependentIPs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 5
	rl := api.NewRateLimiter(cfg)

	// Drain IP-A
	for i := 0; i < cfg.RateLimitPerIP; i++ {
		if !rl.Allow("10.0.0.1") {
			t.Fatalf("IP-A token %d not allowed", i)
		}
	}
	// IP-A exhausted
	if rl.Allow("10.0.0.1") {
		t.Fatal("IP-A should be exhausted")
	}
	// IP-B should still have full bucket
	if !rl.Allow("10.0.0.2") {
		t.Fatal("IP-B should be independent and allowed")
	}
}

func TestRateLimiterCleanupRemovesExpired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 100
	rl := api.NewRateLimiter(cfg)
	rl.SetCleanupPolicy(10*time.Millisecond, 50*time.Millisecond)

	// Create entry
	rl.Allow("10.0.0.99")

	// Wait past TTL + cleanup interval
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup by calling Allow (different IP)
	rl.Allow("10.0.0.1")

	// Original IP should have full bucket (entry cleaned, fresh on next access)
	for i := 0; i < cfg.RateLimitPerIP; i++ {
		if !rl.Allow("10.0.0.99") {
			t.Fatalf("token %d not allowed after cleanup", i)
		}
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	cfg := config.DefaultConfig()
	rl := api.NewRateLimiter(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rl.Allow(ip)
			}
		}(fmt.Sprintf("10.0.0.%d", i))
	}
	wg.Wait()
	// No panic or race detected = pass (run with -race)
}

func TestRateLimiterCleanupConcurrentAllowStress(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 5000
	cfg.RateLimitPerIP = 5000
	rl := api.NewRateLimiter(cfg)
	rl.SetCleanupPolicy(2*time.Millisecond, 4*time.Millisecond)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	workers := 12
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			n := 0
			for {
				select {
				case <-stop:
					return
				default:
					ip := fmt.Sprintf("10.0.%d.%d", worker, n%64)
					rl.Allow(ip)
					n++
				}
			}
		}(i)
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestRateLimiterBoundedIPCardinality(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 1000
	rl := api.NewRateLimiter(cfg)
	rl.SetMaxIPEntries(2)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("first IP should be allowed")
	}
	if !rl.Allow("10.0.0.2") {
		t.Fatal("second IP should be allowed")
	}
	if rl.Allow("10.0.0.3") {
		t.Fatal("third unique IP should be rejected once map is full")
	}
}

func TestRateLimiterNoGlobalBurnOnIPReject(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 2
	cfg.RateLimitPerIP = 1
	rl := api.NewRateLimiter(cfg)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("first request should pass")
	}
	if rl.Allow("10.0.0.1") {
		t.Fatal("second request from same IP should fail per-IP limit")
	}
	// If global token was not refunded on IP reject, this request would fail.
	if !rl.Allow("10.0.0.2") {
		t.Fatal("global token should be available for different IP")
	}
}
