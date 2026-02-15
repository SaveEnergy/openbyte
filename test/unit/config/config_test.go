package config_test

import (
	"os"
	"path/filepath"
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
	if err := cfg.LoadFromEnv(); err == nil {
		t.Fatal("expected error for non-numeric GLOBAL_RATE_LIMIT")
	}
}

func TestConfigLoadGlobalRateLimitEnvNonPositive(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "-5")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err == nil {
		t.Fatal("expected error for negative GLOBAL_RATE_LIMIT")
	}
}

func TestConfigValidateMaxTestDurationPositive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxTestDuration = 10 * time.Second

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigValidatePortRange(t *testing.T) {
	cfg := config.DefaultConfig()

	// Valid port
	cfg.Port = "8080"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("port 8080 should be valid: %v", err)
	}

	// Port 0 — invalid
	cfg.Port = "0"
	if err := cfg.Validate(); err == nil {
		t.Fatal("port 0 should be invalid")
	}

	// Port too high
	cfg.Port = "99999"
	if err := cfg.Validate(); err == nil {
		t.Fatal("port 99999 should be invalid")
	}

	// Non-numeric port (already caught by LoadFromEnv, but Validate should also catch)
	cfg.Port = "abc"
	if err := cfg.Validate(); err == nil {
		t.Fatal("port 'abc' should be invalid")
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

func TestDataDirValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty DataDir should be invalid")
	}
	cfg.DataDir = "/tmp/test"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid DataDir should pass: %v", err)
	}
}

func TestMaxStoredResultsValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxStoredResults = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("MaxStoredResults=0 should be invalid")
	}
	cfg.MaxStoredResults = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("MaxStoredResults=-1 should be invalid")
	}
	cfg.MaxStoredResults = 100
	if err := cfg.Validate(); err != nil {
		t.Fatalf("MaxStoredResults=100 should be valid: %v", err)
	}
}

func TestValidateTrustedProxyCIDRsWithoutTrustProxyHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = false
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8", "192.168.0.0/16"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid trusted proxy CIDRs should pass even when trust proxy headers disabled: %v", err)
	}
}

func TestValidateTrustedProxyCIDRsRejectsInvalidCIDR(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = false
	cfg.TrustedProxyCIDRs = []string{"not-a-cidr"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid trusted proxy CIDR should fail validation")
	}
}

func TestValidateRegistryIntervalWhenRegistryEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RegistryEnabled = true
	cfg.RegistryURL = "https://registry.example.com"
	cfg.RegistryInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("registry interval <= 0 should fail when registry enabled")
	}

	cfg.RegistryInterval = 5 * time.Second
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid registry interval should pass: %v", err)
	}
}

func TestValidateTrustProxyHeadersRequiresTrustedCIDRs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when trust proxy headers enabled without trusted CIDRs")
	}

	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected trusted CIDR to satisfy validation: %v", err)
	}
}

func TestConfigValidateRegistryURLRequired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RegistryEnabled = true
	cfg.RegistryInterval = 5 * time.Second
	cfg.RegistryURL = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when registry enabled without URL")
	}
}

func TestConfigValidateTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TLSCertFile = "/tmp/cert.pem"
	cfg.TLSKeyFile = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when only one TLS file is set")
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o644); err != nil {
		t.Fatalf("write key: %v", err)
	}
	cfg = config.DefaultConfig()
	cfg.TLSCertFile = certPath
	cfg.TLSKeyFile = keyPath
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid TLS file pair: %v", err)
	}

	cfg = config.DefaultConfig()
	cfg.TLSCertFile = certPath
	cfg.TLSKeyFile = filepath.Join(dir, "missing-key.pem")
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing TLS key file")
	}
}

func TestConfigValidateCapacityGbpsPositive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.CapacityGbps = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for non-positive capacity gbps")
	}
}

func TestValidateMaxConcurrentPerIPWithinBounds(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentTests = 10
	cfg.MaxConcurrentPerIP = 11
	if err := cfg.Validate(); err == nil {
		t.Fatal("max concurrent per IP > max concurrent tests should fail validation")
	}

	cfg.MaxConcurrentPerIP = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("max concurrent per IP <= 0 should fail validation")
	}

	cfg.MaxConcurrentPerIP = 5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("bounded max concurrent per IP should pass: %v", err)
	}
}
