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
	clientIPResolver *ClientIPResolver
}

type IPLimit struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

func NewRateLimiter(cfg *config.Config) *RateLimiter {
	return &RateLimiter{
		config:           cfg,
		ipLimits:         make(map[string]*IPLimit),
		globalTokens:     cfg.GlobalRateLimit,
		globalLastRefill: time.Now(),
		lastCleanup:      time.Now(),
		cleanupInterval:  5 * time.Minute,
		ipLimitTTL:       10 * time.Minute,
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
			rl.globalLastRefill = now
		}
	}

	if rl.globalTokens > 0 {
		rl.globalTokens--
		return true
	}

	return false
}

func (rl *RateLimiter) allowIP(ip string) bool {
	rl.ipMu.Lock()
	now := time.Now()
	if rl.cleanupInterval > 0 && rl.ipLimitTTL > 0 && now.Sub(rl.lastCleanup) >= rl.cleanupInterval {
		for key, limit := range rl.ipLimits {
			limit.mu.Lock()
			lastRefill := limit.lastRefill
			limit.mu.Unlock()
			if now.Sub(lastRefill) >= rl.ipLimitTTL {
				delete(rl.ipLimits, key)
			}
		}
		rl.lastCleanup = now
	}
	limit, exists := rl.ipLimits[ip]
	if !exists {
		limit = &IPLimit{
			tokens:     rl.config.RateLimitPerIP,
			lastRefill: now,
		}
		rl.ipLimits[ip] = limit
	}
	rl.ipMu.Unlock()

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
			limit.lastRefill = now
		}
	}

	if limit.tokens > 0 {
		limit.tokens--
		return true
	}

	return false
}

// skipRateLimitPaths are endpoints that should not be rate limited
// These are high-frequency speedtest endpoints
var skipRateLimitPaths = map[string]bool{
	"/api/v1/download": true,
	"/api/v1/upload":   true,
	"/api/v1/ping":     true,
}

func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for speedtest endpoints
			if skipRateLimitPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ip := limiter.ClientIP(r)

			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
