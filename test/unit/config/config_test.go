package config_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestConfigValidateGlobalRateLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.GlobalRateLimit = 0

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for global rate limit <= 0")
	}
}

func TestConfigValidateGlobalRateLimitLessThanPerIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RateLimitPerIP = 200
	cfg.GlobalRateLimit = 100

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for global rate limit < rate limit per IP")
	}
}

func TestConfigValidateMaxTestDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 0

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for max test duration <= 0")
	}
}

func TestConfigLoadGlobalRateLimitEnv(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "250")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalRateLimit != 250 {
		t.Fatalf("expected global rate limit 250, got %d", cfg.GlobalRateLimit)
	}
}

func TestConfigLoadGlobalRateLimitEnvInvalid(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "not-a-number")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalRateLimit != 1000 {
		t.Fatalf("expected default global rate limit to remain, got %d", cfg.GlobalRateLimit)
	}
}

func TestConfigLoadGlobalRateLimitEnvNonPositive(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "-5")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GlobalRateLimit != 1000 {
		t.Fatalf("expected default global rate limit to remain, got %d", cfg.GlobalRateLimit)
	}
}

func TestConfigValidateMaxTestDurationPositive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 10 * time.Second

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
