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
	return newRateLimiter(cfg, NewClientIPResolver(cfg))
}

func newRateLimiter(cfg *config.Config, resolver *ClientIPResolver) *RateLimiter {
	maxIPEntries := max(cfg.GlobalRateLimit*20, 10000)
	return &RateLimiter{
		rateLimitPerIP:   cfg.RateLimitPerIP,
		globalRateLimit:  cfg.GlobalRateLimit,
		ipLimits:         make(map[string]*IPLimit),
		globalTokens:     cfg.GlobalRateLimit,
		globalLastRefill: time.Now(),
		lastCleanup:      time.Now(),
		cleanupInterval:  5 * time.Minute,
		ipLimitTTL:       10 * time.Minute,
		maxIPEntries:     maxIPEntries,
		clientIPResolver: resolver,
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	if !rl.allowGlobal() {
		return false
	}
	if !rl.allowIP(ip) {
		rl.refundGlobal()
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
