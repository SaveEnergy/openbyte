package api_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const (
	ipPrimary       = "10.0.0.1"
	ipSecondary     = "10.0.0.2"
	cleanupTestIP   = "10.0.0.99"
	thirdUniqueIP   = "10.0.0.3"
	ipPrefixPrimary = "10.0.0.%d"
	ipPrefixWorkers = "10.0.%d.%d"
)

func TestRateLimiterIndependentIPs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 5
	rl := api.NewRateLimiter(cfg)

	// Drain IP-A
	for i := range cfg.RateLimitPerIP {
		if !rl.Allow(ipPrimary) {
			t.Fatalf("IP-A token %d not allowed", i)
		}
	}
	// IP-A exhausted
	if rl.Allow(ipPrimary) {
		t.Fatal("IP-A should be exhausted")
	}
	// IP-B should still have full bucket
	if !rl.Allow(ipSecondary) {
		t.Fatal("IP-B should be independent and allowed")
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	cfg := config.DefaultConfig()
	rl := api.NewRateLimiter(cfg)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			for range 100 {
				rl.Allow(ip)
			}
		}(fmt.Sprintf(ipPrefixPrimary, i))
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

	var wg sync.WaitGroup
	workers := 12
	for i := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for n := range 2_000 {
				ip := fmt.Sprintf(ipPrefixWorkers, worker, n%64)
				rl.Allow(ip)
			}
		}(i)
	}
	wg.Wait()
}

func TestRateLimiterBoundedIPCardinality(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 1000
	rl := api.NewRateLimiter(cfg)
	rl.SetMaxIPEntries(2)

	if !rl.Allow(ipPrimary) {
		t.Fatal("first IP should be allowed")
	}
	if !rl.Allow(ipSecondary) {
		t.Fatal("second IP should be allowed")
	}
	if rl.Allow(thirdUniqueIP) {
		t.Fatal("third unique IP should be rejected once map is full")
	}
}

func TestRateLimiterNoGlobalBurnOnIPReject(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 2
	cfg.RateLimitPerIP = 1
	rl := api.NewRateLimiter(cfg)

	if !rl.Allow(ipPrimary) {
		t.Fatal("first request should pass")
	}
	if rl.Allow(ipPrimary) {
		t.Fatal("second request from same IP should fail per-IP limit")
	}
	// If global token was not refunded on IP reject, this request would fail.
	if !rl.Allow(ipSecondary) {
		t.Fatal("global token should be available for different IP")
	}
}
