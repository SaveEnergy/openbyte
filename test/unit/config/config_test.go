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

func TestMaxConcurrentHTTPScalesWithCapacity(t *testing.T) {
	tests := []struct {
		capacity int
		wantMin  int
	}{
		{1, 50},   // floor
		{5, 50},   // still floor
		{10, 80},  // 10 * 8
		{25, 200}, // 25 * 8
	}
	for _, tt := range tests {
		cfg := config.DefaultConfig()
		cfg.CapacityGbps = tt.capacity
		got := cfg.MaxConcurrentHTTP()
		if got != tt.wantMin {
			t.Errorf("CapacityGbps=%d: MaxConcurrentHTTP()=%d, want %d", tt.capacity, got, tt.wantMin)
		}
	}
}

func TestMaxStreamsValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxStreams = 32
	if err := cfg.Validate(); err != nil {
		t.Fatalf("MaxStreams=32 should be valid: %v", err)
	}
	cfg.MaxStreams = 64
	if err := cfg.Validate(); err != nil {
		t.Fatalf("MaxStreams=64 should be valid: %v", err)
	}
	cfg.MaxStreams = 65
	if err := cfg.Validate(); err == nil {
		t.Fatalf("MaxStreams=65 should be invalid")
	}
}
