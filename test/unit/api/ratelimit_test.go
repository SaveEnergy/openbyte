package api_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const (
	ipPrimary       = "10.0.0.1"
	ipSecondary     = "10.0.0.2"
	ipPrefixPrimary = "10.0.0.%d"
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
