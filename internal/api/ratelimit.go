package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

type RateLimiter struct {
	config           *config.Config
	ipLimits         map[string]*IPLimit
	ipMu             sync.RWMutex
	globalTokens     int
	globalMu         sync.Mutex
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
	mu         sync.Mutex
}

func NewRateLimiter(cfg *config.Config) *RateLimiter {
	maxIPEntries := cfg.GlobalRateLimit * 20
	if maxIPEntries < 10000 {
		maxIPEntries = 10000
	}
	return &RateLimiter{
		config:           cfg,
		ipLimits:         make(map[string]*IPLimit),
		globalTokens:     cfg.GlobalRateLimit,
		globalLastRefill: time.Now(),
		lastCleanup:      time.Now(),
		cleanupInterval:  5 * time.Minute,
		ipLimitTTL:       10 * time.Minute,
		maxIPEntries:     maxIPEntries,
		clientIPResolver: NewClientIPResolver(cfg),
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	if !rl.allowGlobal() {
		return false
	}
	if !rl.allowIP(ip) {
		return false
	}
	return true
}

func (rl *RateLimiter) ClientIP(r *http.Request) string {
	if rl.clientIPResolver != nil {
		return rl.clientIPResolver.FromRequest(r)
	}
	return ipString(parseRemoteIP(r.RemoteAddr))
}

// SetCleanupPolicy overrides cleanup interval and TTL (mainly for tests).
func (rl *RateLimiter) SetCleanupPolicy(cleanupInterval, ipLimitTTL time.Duration) {
	rl.ipMu.Lock()
	defer rl.ipMu.Unlock()
	rl.cleanupInterval = cleanupInterval
	rl.ipLimitTTL = ipLimitTTL
	rl.lastCleanup = time.Now()
}

// SetMaxIPEntries overrides the per-IP cardinality cap (mainly for tests).
func (rl *RateLimiter) SetMaxIPEntries(limit int) {
	rl.ipMu.Lock()
	defer rl.ipMu.Unlock()
	if limit > 0 {
		rl.maxIPEntries = limit
	}
}

func (rl *RateLimiter) allowGlobal() bool {
	rl.globalMu.Lock()
	defer rl.globalMu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.globalLastRefill)

	if elapsed >= time.Second {
		tokensToAdd := int(elapsed.Seconds() * float64(rl.config.GlobalRateLimit) / 60.0)
		if tokensToAdd > 0 {
			rl.globalTokens += tokensToAdd
			if rl.globalTokens > rl.config.GlobalRateLimit {
				rl.globalTokens = rl.config.GlobalRateLimit
			}
			// Advance only by time consumed to preserve fractional remainder
			consumed := time.Duration(float64(tokensToAdd) / float64(rl.config.GlobalRateLimit) * 60.0 * float64(time.Second))
			rl.globalLastRefill = rl.globalLastRefill.Add(consumed)
		}
	}

	if rl.globalTokens > 0 {
		rl.globalTokens--
		return true
	}

	return false
}

func (rl *RateLimiter) allowIP(ip string) bool {
	now := time.Now()
	shouldCleanup := false
	rl.ipMu.Lock()
	if rl.cleanupInterval > 0 && rl.ipLimitTTL > 0 && now.Sub(rl.lastCleanup) >= rl.cleanupInterval {
		rl.lastCleanup = now
		shouldCleanup = true
	}
	limit, exists := rl.ipLimits[ip]
	if !exists {
		if rl.maxIPEntries > 0 && len(rl.ipLimits) >= rl.maxIPEntries {
			rl.ipMu.Unlock()
			if shouldCleanup {
				rl.cleanupExpiredIPLimits(now)
			}
			return false
		}
		limit = &IPLimit{
			tokens:     rl.config.RateLimitPerIP,
			lastRefill: now,
		}
		rl.ipLimits[ip] = limit
	}
	rl.ipMu.Unlock()
	if shouldCleanup {
		rl.cleanupExpiredIPLimits(now)
	}

	limit.mu.Lock()
	defer limit.mu.Unlock()

	elapsed := now.Sub(limit.lastRefill)

	if elapsed >= time.Second {
		tokensToAdd := int(elapsed.Seconds() * float64(rl.config.RateLimitPerIP) / 60.0)
		if tokensToAdd > 0 {
			limit.tokens += tokensToAdd
			if limit.tokens > rl.config.RateLimitPerIP {
				limit.tokens = rl.config.RateLimitPerIP
			}
			consumed := time.Duration(float64(tokensToAdd) / float64(rl.config.RateLimitPerIP) * 60.0 * float64(time.Second))
			limit.lastRefill = limit.lastRefill.Add(consumed)
		}
	}

	if limit.tokens > 0 {
		limit.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanupExpiredIPLimits(now time.Time) {
	rl.ipMu.RLock()
	ttl := rl.ipLimitTTL
	expired := make([]string, 0)
	for key, limit := range rl.ipLimits {
		limit.mu.Lock()
		lastRefill := limit.lastRefill
		limit.mu.Unlock()
		if now.Sub(lastRefill) >= ttl {
			expired = append(expired, key)
		}
	}
	rl.ipMu.RUnlock()

	if len(expired) == 0 {
		return
	}

	rl.ipMu.Lock()
	for _, key := range expired {
		limit, exists := rl.ipLimits[key]
		if !exists {
			continue
		}
		limit.mu.Lock()
		isExpired := now.Sub(limit.lastRefill) >= ttl
		limit.mu.Unlock()
		if isExpired {
			delete(rl.ipLimits, key)
		}
	}
	rl.ipMu.Unlock()
}

// skipRateLimitPaths are endpoints that should not be rate limited
// These are high-frequency speedtest endpoints
var skipRateLimitPaths = map[string]bool{
	"/api/v1/download": true,
	"/api/v1/upload":   true,
	"/api/v1/ping":     true,
}
