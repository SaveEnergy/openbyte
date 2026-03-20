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
	maxIPEntries := max(cfg.GlobalRateLimit*20, 10000)
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

// skipRateLimitPaths are endpoints that should not be rate limited
// These are high-frequency speedtest endpoints
var skipRateLimitPaths = map[string]bool{
	"/api/v1/download": true,
	"/api/v1/upload":   true,
	"/api/v1/ping":     true,
}
