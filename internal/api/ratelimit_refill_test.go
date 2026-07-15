package api

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestRefillTokensLowRates(t *testing.T) {
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
			start := time.Unix(1, 0)
			tokens := 0
			lastRefill := start

			refillTokens(&tokens, &lastRefill, test.rate, start.Add(test.elapsed))

			if tokens != 1 {
				t.Fatalf("tokens = %d, want 1", tokens)
			}
		})
	}
}

func TestRefillTokensPreservesFractionalRemainder(t *testing.T) {
	start := time.Unix(1, 0)
	tokens := 0
	lastRefill := start

	refillTokens(&tokens, &lastRefill, 10, start.Add(7*time.Second))
	if tokens != 1 {
		t.Fatalf("first refill tokens = %d, want 1", tokens)
	}
	if want := start.Add(6 * time.Second); !lastRefill.Equal(want) {
		t.Fatalf("first refill timestamp = %v, want %v", lastRefill, want)
	}

	refillTokens(&tokens, &lastRefill, 10, start.Add(12*time.Second))
	if tokens != 2 {
		t.Fatalf("second refill tokens = %d, want 2", tokens)
	}
	if want := start.Add(12 * time.Second); !lastRefill.Equal(want) {
		t.Fatalf("second refill timestamp = %v, want %v", lastRefill, want)
	}
}

func TestCleanupExpiredIPLimits(t *testing.T) {
	cfg := config.DefaultConfig()
	rl := NewRateLimiter(cfg)
	now := time.Unix(100, 0)
	rl.ipLimitTTL = time.Minute
	rl.ipLimits["expired"] = &IPLimit{tokens: 1, lastRefill: now.Add(-2 * time.Minute)}
	rl.ipLimits["current"] = &IPLimit{tokens: 1, lastRefill: now}

	rl.mu.Lock()
	rl.cleanupExpiredIPLimits(now)
	rl.mu.Unlock()

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
	globalTokens := limiter.globalTokens
	if limiter.Allow("10.0.0.3") {
		t.Fatal("third unique IP should be rejected once map is full")
	}
	if limiter.globalTokens != globalTokens {
		t.Fatalf("capacity rejection consumed a global token: got %d, want %d", limiter.globalTokens, globalTokens)
	}
	if got := len(limiter.ipLimits); got != limiter.maxIPEntries {
		t.Fatalf("IP bucket count = %d, want %d", got, limiter.maxIPEntries)
	}
}

func TestRateLimiterCleanupReclaimsCapacityBeforeInsert(t *testing.T) {
	cfg := config.DefaultConfig()
	limiter := NewRateLimiter(cfg)
	now := time.Now()
	limiter.maxIPEntries = 1
	limiter.cleanupInterval = time.Minute
	limiter.ipLimitTTL = time.Minute
	limiter.lastCleanup = now.Add(-2 * time.Minute)
	limiter.ipLimits["expired"] = &IPLimit{
		tokens:     1,
		lastRefill: now.Add(-2 * time.Minute),
	}

	if !limiter.Allow("current") {
		t.Fatal("new IP should be admitted after due cleanup reclaims capacity")
	}
	if _, exists := limiter.ipLimits["expired"]; exists {
		t.Fatal("expired IP bucket was not removed")
	}
	if _, exists := limiter.ipLimits["current"]; !exists {
		t.Fatal("current IP bucket was not created")
	}
	if got := len(limiter.ipLimits); got != 1 {
		t.Fatalf("IP bucket count = %d, want 1", got)
	}
}

func TestRateLimiterGlobalExhaustionDoesNotTrackIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 1
	cfg.RateLimitPerIP = 1
	limiter := NewRateLimiter(cfg)

	if !limiter.Allow("10.0.0.1") {
		t.Fatal("first IP should be allowed")
	}
	if limiter.Allow("10.0.0.2") {
		t.Fatal("second IP should be rejected by the global limit")
	}
	if _, exists := limiter.ipLimits["10.0.0.2"]; exists {
		t.Fatal("global rejection should not create an IP bucket")
	}
}

func TestRateLimiterConcurrentExactLimits(t *testing.T) {
	const (
		ipCount       = 8
		perIPLimit    = 7
		attemptsPerIP = 50
	)

	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = ipCount * perIPLimit
	cfg.RateLimitPerIP = perIPLimit
	limiter := NewRateLimiter(cfg)
	future := time.Now().Add(time.Hour)
	limiter.globalLastRefill = future
	limiter.cleanupInterval = 0

	addresses := make([]string, ipCount)
	for i := range ipCount {
		addresses[i] = fmt.Sprintf("10.0.1.%d", i)
		limiter.ipLimits[addresses[i]] = &IPLimit{
			tokens:     perIPLimit,
			lastRefill: future,
		}
	}

	var total atomic.Int64
	perIP := make([]atomic.Int64, ipCount)
	var wg sync.WaitGroup
	for i := range ipCount {
		for range attemptsPerIP {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if limiter.Allow(addresses[i]) {
					perIP[i].Add(1)
					total.Add(1)
				}
			}()
		}
	}
	wg.Wait()

	if got, want := total.Load(), int64(ipCount*perIPLimit); got != want {
		t.Fatalf("allowed requests = %d, want %d", got, want)
	}
	for i := range ipCount {
		if got := perIP[i].Load(); got != perIPLimit {
			t.Fatalf("IP %s allowed requests = %d, want %d", addresses[i], got, perIPLimit)
		}
	}
}
