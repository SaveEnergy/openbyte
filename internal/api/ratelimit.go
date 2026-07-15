package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

type RateLimiter struct {
	rateLimitPerIP   int
	globalRateLimit  int
	ipLimits         map[string]*IPLimit
	mu               sync.Mutex
	globalTokens     int
	globalLastRefill time.Time
	lastCleanup      time.Time
	cleanupInterval  time.Duration
	ipLimitTTL       time.Duration
	maxIPEntries     int
	clientIPResolver *ClientIPResolver
}

type IPLimit struct {
	tokens     int
	lastRefill time.Time
}

func NewRateLimiter(cfg *config.Config) *RateLimiter {
	return newRateLimiter(cfg, NewClientIPResolver(cfg))
}

func newRateLimiter(cfg *config.Config, resolver *ClientIPResolver) *RateLimiter {
	maxIPEntries := max(cfg.GlobalRateLimit*20, 10000)
	now := time.Now()
	return &RateLimiter{
		rateLimitPerIP:   cfg.RateLimitPerIP,
		globalRateLimit:  cfg.GlobalRateLimit,
		ipLimits:         make(map[string]*IPLimit),
		globalTokens:     cfg.GlobalRateLimit,
		globalLastRefill: now,
		lastCleanup:      now,
		cleanupInterval:  5 * time.Minute,
		ipLimitTTL:       10 * time.Minute,
		maxIPEntries:     maxIPEntries,
		clientIPResolver: resolver,
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	refillTokens(&rl.globalTokens, &rl.globalLastRefill, rl.globalRateLimit, now)
	if rl.globalTokens <= 0 {
		return false
	}

	if rl.cleanupInterval > 0 && rl.ipLimitTTL > 0 && now.Sub(rl.lastCleanup) >= rl.cleanupInterval {
		rl.cleanupExpiredIPLimits(now)
		rl.lastCleanup = now
	}

	limit, exists := rl.ipLimits[ip]
	if !exists {
		if rl.maxIPEntries > 0 && len(rl.ipLimits) >= rl.maxIPEntries {
			return false
		}
		limit = &IPLimit{
			tokens:     rl.rateLimitPerIP,
			lastRefill: now,
		}
		rl.ipLimits[ip] = limit
	}

	refillTokens(&limit.tokens, &limit.lastRefill, rl.rateLimitPerIP, now)
	if limit.tokens <= 0 {
		return false
	}

	rl.globalTokens--
	limit.tokens--
	return true
}

func refillTokens(tokens *int, lastRefill *time.Time, rate int, now time.Time) {
	elapsed := now.Sub(*lastRefill)
	if elapsed < time.Second {
		return
	}
	tokensToAdd := int(elapsed.Seconds() * float64(rate) / 60.0)
	if tokensToAdd <= 0 {
		return
	}
	*tokens += tokensToAdd
	if *tokens > rate {
		*tokens = rate
	}
	consumed := time.Duration(float64(tokensToAdd) / float64(rate) * 60.0 * float64(time.Second))
	*lastRefill = (*lastRefill).Add(consumed)
}

// cleanupExpiredIPLimits removes stale buckets while Allow holds rl.mu.
func (rl *RateLimiter) cleanupExpiredIPLimits(now time.Time) {
	for key, limit := range rl.ipLimits {
		if now.Sub(limit.lastRefill) >= rl.ipLimitTTL {
			delete(rl.ipLimits, key)
		}
	}
}

func (rl *RateLimiter) ClientIP(r *http.Request) string {
	if rl.clientIPResolver != nil {
		return rl.clientIPResolver.FromRequest(r)
	}
	return ipString(parseRemoteIP(r.RemoteAddr))
}
