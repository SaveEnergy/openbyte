package api

import "time"

func (rl *RateLimiter) allowIP(ip string) bool {
	now := time.Now()
	limit, shouldCleanup, ok := rl.getOrCreateIPLimit(ip, now)
	if !ok {
		return false
	}
	if shouldCleanup {
		rl.cleanupExpiredIPLimits(now)
	}

	limit.mu.Lock()
	defer limit.mu.Unlock()
	rl.refillIPTokens(limit, now)
	if limit.tokens <= 0 {
		return false
	}
	limit.tokens--
	return true
}

func (rl *RateLimiter) getOrCreateIPLimit(ip string, now time.Time) (*IPLimit, bool, bool) {
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
			return nil, shouldCleanup, false
		}
		limit = &IPLimit{
			tokens:     rl.config.RateLimitPerIP,
			lastRefill: now,
		}
		rl.ipLimits[ip] = limit
	}
	rl.ipMu.Unlock()
	return limit, shouldCleanup, true
}

func (rl *RateLimiter) refillIPTokens(limit *IPLimit, now time.Time) {
	elapsed := now.Sub(limit.lastRefill)
	if elapsed < time.Second {
		return
	}
	tokensToAdd := int(elapsed.Seconds() * float64(rl.config.RateLimitPerIP) / 60.0)
	if tokensToAdd <= 0 {
		return
	}
	limit.tokens += tokensToAdd
	if limit.tokens > rl.config.RateLimitPerIP {
		limit.tokens = rl.config.RateLimitPerIP
	}
	consumed := time.Duration(float64(tokensToAdd) / float64(rl.config.RateLimitPerIP) * 60.0 * float64(time.Second))
	limit.lastRefill = limit.lastRefill.Add(consumed)
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
