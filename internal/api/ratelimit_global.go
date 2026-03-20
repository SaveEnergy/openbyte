package api

import "time"

func (rl *RateLimiter) allowGlobal() bool {
	rl.globalMu.Lock()
	defer rl.globalMu.Unlock()

	now := time.Now()
	rl.refillGlobalTokens(now)
	return rl.consumeGlobalToken()
}

func (rl *RateLimiter) refillGlobalTokens(now time.Time) {
	elapsed := now.Sub(rl.globalLastRefill)
	if elapsed < time.Second {
		return
	}
	tokensToAdd := int(elapsed.Seconds() * float64(rl.config.GlobalRateLimit) / 60.0)
	if tokensToAdd <= 0 {
		return
	}
	rl.globalTokens += tokensToAdd
	if rl.globalTokens > rl.config.GlobalRateLimit {
		rl.globalTokens = rl.config.GlobalRateLimit
	}
	consumed := time.Duration(float64(tokensToAdd) / float64(rl.config.GlobalRateLimit) * 60.0 * float64(time.Second))
	rl.globalLastRefill = rl.globalLastRefill.Add(consumed)
}

func (rl *RateLimiter) consumeGlobalToken() bool {
	if rl.globalTokens <= 0 {
		return false
	}
	rl.globalTokens--
	return true
}

func (rl *RateLimiter) refundGlobal() {
	rl.globalMu.Lock()
	defer rl.globalMu.Unlock()
	if rl.globalTokens < rl.config.GlobalRateLimit {
		rl.globalTokens++
	}
}
