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
	loopbackIP      = "127.0.0.1"
	ipPrimary       = "10.0.0.1"
	ipSecondary     = "10.0.0.2"
	cleanupTestIP   = "10.0.0.99"
	thirdUniqueIP   = "10.0.0.3"
	ipPrefixPrimary = "10.0.0.%d"
	ipPrefixWorkers = "10.0.%d.%d"
)

func waitUntilAllowed(t *testing.T, timeout time.Duration, allow func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if allow() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

func drainGlobalTokens(t *testing.T, rl *api.RateLimiter, ip string, count int) {
	t.Helper()
	for i := range count {
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

	ip := loopbackIP
	drainGlobalTokens(t, rl, ip, cfg.GlobalRateLimit)

	waitUntilAllowed(t, 3*time.Second, func() bool { return rl.Allow(ip) },
		"expected global refill to allow request at low rate")
}

func TestRateLimiterGlobalRefillVeryLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 10
	cfg.RateLimitPerIP = 1000
	rl := api.NewRateLimiter(cfg)

	ip := loopbackIP
	drainGlobalTokens(t, rl, ip, cfg.GlobalRateLimit)

	waitUntilAllowed(t, 7*time.Second, func() bool { return rl.Allow(ip) },
		"expected global refill to allow request at very low rate")
}

func TestRateLimiterIPRefillLowRate(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 30
	rl := api.NewRateLimiter(cfg)

	ip := loopbackIP
	for i := range cfg.RateLimitPerIP {
		if !rl.Allow(ip) {
			t.Fatalf("token %d not allowed", i)
		}
	}

	waitUntilAllowed(t, 3*time.Second, func() bool { return rl.Allow(ip) },
		"expected per-ip refill to allow request at low rate")
}

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

func TestRateLimiterCleanupRemovesExpired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1000
	cfg.RateLimitPerIP = 100
	rl := api.NewRateLimiter(cfg)
	rl.SetCleanupPolicy(10*time.Millisecond, 50*time.Millisecond)

	// Create entry
	rl.Allow(cleanupTestIP)

	// Wait past TTL + cleanup interval
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup by calling Allow (different IP)
	rl.Allow(ipPrimary)

	// Original IP should have full bucket (entry cleaned, fresh on next access)
	for i := range cfg.RateLimitPerIP {
		if !rl.Allow(cleanupTestIP) {
			t.Fatalf("token %d not allowed after cleanup", i)
		}
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

	stop := make(chan struct{})
	var wg sync.WaitGroup
	workers := 12
	for i := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			n := 0
			for {
				select {
				case <-stop:
					return
				default:
					ip := fmt.Sprintf(ipPrefixWorkers, worker, n%64)
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
