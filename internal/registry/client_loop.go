package registry

import (
	cryptorand "crypto/rand"
	"math/big"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

func (c *Client) heartbeatLoop(getActiveTests func() int) {
	defer c.wg.Done()

	baseInterval := c.config.RegistryInterval
	if baseInterval <= 0 {
		baseInterval = defaultRegistryInterval
	}
	timer := time.NewTimer(baseInterval)
	defer timer.Stop()
	failureCount := 0

	for {
		select {
		case <-c.stopCh:
			return
		case <-timer.C:
			if err := c.heartbeat(getActiveTests()); err != nil {
				failureCount++
				delay := c.nextHeartbeatDelay(baseInterval, failureCount)
				c.logger.Error("Registry heartbeat failed",
					logging.Field{Key: "error", Value: err},
					logging.Field{Key: "retry_in_ms", Value: delay.Milliseconds()})
				timer.Reset(delay)
				continue
			}
			failureCount = 0
			timer.Reset(c.addJitter(baseInterval))
		}
	}
}

func (c *Client) nextHeartbeatDelay(baseInterval time.Duration, failures int) time.Duration {
	backoff := baseInterval
	for i := 1; i < failures && i < maxHeartbeatBackoffFactor; i++ {
		backoff *= 2
	}
	maxBackoff := baseInterval * maxHeartbeatBackoffFactor
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return c.addJitter(backoff)
}

func (c *Client) addJitter(base time.Duration) time.Duration {
	if base <= minimumJitterWindow {
		return base
	}
	jitterWindow := base / heartbeatJitterDivisor
	if jitterWindow <= 0 {
		return base
	}
	max := int64(jitterWindow*2 + 1)
	offset := c.randomInt63n(max) - int64(jitterWindow)
	jittered := base + time.Duration(offset)
	if jittered <= 0 {
		return base
	}
	return jittered
}

func (c *Client) randomInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	v, err := cryptorand.Int(cryptorand.Reader, big.NewInt(n))
	if err != nil {
		// Deterministic fallback: keep behavior stable if entropy is unavailable.
		return n / 2
	}
	return v.Int64()
}
