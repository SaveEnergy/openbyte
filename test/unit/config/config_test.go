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

	if cfg.Validate() == nil {
		t.Fatalf("expected error for global rate limit <= 0")
	}
}

func TestConfigValidateGlobalRateLimitLessThanPerIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RateLimitPerIP = 200
	cfg.GlobalRateLimit = 100

	if cfg.Validate() == nil {
		t.Fatalf("expected error for global rate limit < rate limit per IP")
	}
}

func TestConfigValidateMaxTestDuration(t *testing.T) {
	for _, duration := range []time.Duration{0, -time.Second, 500 * time.Millisecond, 1500 * time.Millisecond} {
		cfg := config.DefaultConfig()
		cfg.MaxTestDuration = duration
		if cfg.Validate() == nil {
			t.Fatalf("expected error for max test duration %v", duration)
		}
	}
}

func TestConfigLoadRejectsFractionalMaxTestDuration(t *testing.T) {
	for _, raw := range []string{"500ms", "1500ms"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("MAX_TEST_DURATION", raw)

			cfg := config.DefaultConfig()
			if err := cfg.LoadFromEnv(); err == nil {
				t.Fatalf("expected MAX_TEST_DURATION=%s to be rejected", raw)
			}
		})
	}
}

func TestConfigLoadAcceptsWholeSecondMaxTestDuration(t *testing.T) {
	t.Setenv("MAX_TEST_DURATION", "2s")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load MAX_TEST_DURATION: %v", err)
	}
	if cfg.MaxTestDuration != 2*time.Second {
		t.Fatalf("max test duration = %v, want 2s", cfg.MaxTestDuration)
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

func TestConfigLoadServerNameEnv(t *testing.T) {
	t.Setenv("SERVER_NAME", "Frankfurt 10G")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerName != "Frankfurt 10G" {
		t.Fatalf("server name = %q, want Frankfurt 10G", cfg.ServerName)
	}
}

func TestConfigLoadGlobalRateLimitEnvInvalid(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "not-a-number")

	cfg := config.DefaultConfig()
	if cfg.LoadFromEnv() == nil {
		t.Fatal("expected error for non-numeric GLOBAL_RATE_LIMIT")
	}
}

func TestConfigLoadGlobalRateLimitEnvNonPositive(t *testing.T) {
	t.Setenv("GLOBAL_RATE_LIMIT", "-5")

	cfg := config.DefaultConfig()
	if cfg.LoadFromEnv() == nil {
		t.Fatal("expected error for negative GLOBAL_RATE_LIMIT")
	}
}

func TestConfigValidateMaxTestDurationPositive(t *testing.T) {
	for _, duration := range []time.Duration{time.Second, 10 * time.Second, 300 * time.Second} {
		cfg := config.DefaultConfig()
		cfg.MaxTestDuration = duration
		if err := cfg.Validate(); err != nil {
			t.Fatalf("duration %v: unexpected error: %v", duration, err)
		}
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
	if cfg.Validate() == nil {
		t.Fatal("port 0 should be invalid")
	}

	// Port too high
	cfg.Port = "99999"
	if cfg.Validate() == nil {
		t.Fatal("port 99999 should be invalid")
	}

	// Non-numeric port (already caught by LoadFromEnv, but Validate should also catch)
	cfg.Port = "abc"
	if cfg.Validate() == nil {
		t.Fatal("port 'abc' should be invalid")
	}
}

func TestConfigLoadMaxConcurrentTransfersEnv(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.MaxConcurrentTransfers != 200 {
		t.Fatalf("default max concurrent transfers = %d, want 200", cfg.MaxConcurrentTransfers)
	}

	t.Setenv("MAX_CONCURRENT_TRANSFERS", "80")
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load MAX_CONCURRENT_TRANSFERS: %v", err)
	}
	if cfg.MaxConcurrentTransfers != 80 {
		t.Fatalf("max concurrent transfers = %d, want 80", cfg.MaxConcurrentTransfers)
	}
}

func TestConfigLoadRejectsInvalidMaxConcurrentTransfers(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MAX_CONCURRENT_TRANSFERS", value)
			cfg := config.DefaultConfig()
			if err := cfg.LoadFromEnv(); err == nil {
				t.Fatalf("expected MAX_CONCURRENT_TRANSFERS=%q to fail", value)
			}
		})
	}
}

func TestDataDirValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = ""
	if cfg.Validate() == nil {
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
	if cfg.Validate() == nil {
		t.Fatal("MaxStoredResults=0 should be invalid")
	}
	cfg.MaxStoredResults = -1
	if cfg.Validate() == nil {
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
	if cfg.Validate() == nil {
		t.Fatal("invalid trusted proxy CIDR should fail validation")
	}
}

func TestValidateTrustProxyHeadersRequiresTrustedCIDRs(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = nil
	if cfg.Validate() == nil {
		t.Fatal("expected error when trust proxy headers enabled without trusted CIDRs")
	}

	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected trusted CIDR to satisfy validation: %v", err)
	}
}

func TestConfigValidateTLS(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TLSCertFile = "/tmp/cert.pem"
	cfg.TLSKeyFile = ""
	if cfg.Validate() == nil {
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
	if cfg.Validate() == nil {
		t.Fatal("expected error for missing TLS key file")
	}
}

func TestConfigTLSAutoGenIsExplicitOptIn(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.TLSAutoGen {
		t.Fatal("TLSAutoGen should be disabled by default to preserve HTTP dev startup")
	}

	t.Setenv("TLS_AUTO_GEN", "1")
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load env: %v", err)
	}
	if !cfg.TLSAutoGen {
		t.Fatal("TLS_AUTO_GEN=1 should enable generated TLS")
	}
}

func TestConfigLoadHTTP2EnabledEnv(t *testing.T) {
	cfg := config.DefaultConfig()
	if !cfg.HTTP2Enabled {
		t.Fatal("HTTP2Enabled should default to true")
	}

	t.Setenv("HTTP2_ENABLED", "false")
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load env: %v", err)
	}
	if cfg.HTTP2Enabled {
		t.Fatal("HTTP2_ENABLED=false should disable HTTP/2")
	}
}

func TestConfigValidateMaxConcurrentTransfersPositive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentTransfers = 0
	if cfg.Validate() == nil {
		t.Fatal("expected error for non-positive max concurrent transfers")
	}
}

func TestConfigAllowsTransferLimitBelowPerIPLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentTransfers = 50
	cfg.MaxConcurrentPerIP = 64
	if err := cfg.Validate(); err != nil {
		t.Fatalf("independent global and per-IP transfer limits should be valid: %v", err)
	}
}

func TestValidateMaxConcurrentPerIPPositive(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxConcurrentPerIP = 0
	if cfg.Validate() == nil {
		t.Fatal("max concurrent per IP <= 0 should fail validation")
	}

	cfg.MaxConcurrentPerIP = 5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("positive max concurrent per IP should pass: %v", err)
	}
}

func TestConfigLoadImpressumURLEnv(t *testing.T) {
	t.Setenv("IMPRESSUM_URL", " https://legal.example.com/impressum ")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load IMPRESSUM_URL: %v", err)
	}
	if cfg.ImpressumURL != "https://legal.example.com/impressum" {
		t.Fatalf("impressum URL = %q, want trimmed value", cfg.ImpressumURL)
	}
}

func TestConfigValidateImpressumURL(t *testing.T) {
	for _, valid := range []string{"", "https://legal.example.com/impressum", "http://intranet/impressum"} {
		cfg := config.DefaultConfig()
		cfg.ImpressumURL = valid
		if err := cfg.Validate(); err != nil {
			t.Fatalf("impressum URL %q should be valid: %v", valid, err)
		}
	}
	for _, invalid := range []string{"/impressum", "example.com/impressum", "ftp://example.com/impressum", "https://", "https://:443/impressum"} {
		cfg := config.DefaultConfig()
		cfg.ImpressumURL = invalid
		if cfg.Validate() == nil {
			t.Fatalf("impressum URL %q should be invalid", invalid)
		}
	}
}

func TestConfigLoadPrivacyURLEnv(t *testing.T) {
	t.Setenv("PRIVACY_URL", " https://legal.example.com/privacy ")

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		t.Fatalf("load PRIVACY_URL: %v", err)
	}
	if cfg.PrivacyURL != "https://legal.example.com/privacy" {
		t.Fatalf("privacy URL = %q, want trimmed value", cfg.PrivacyURL)
	}
}

func TestConfigValidatePrivacyURL(t *testing.T) {
	for _, valid := range []string{"", "https://legal.example.com/privacy", "http://intranet/privacy"} {
		cfg := config.DefaultConfig()
		cfg.PrivacyURL = valid
		if err := cfg.Validate(); err != nil {
			t.Fatalf("privacy URL %q should be valid: %v", valid, err)
		}
	}
	for _, invalid := range []string{"/privacy", "example.com/privacy", "ftp://example.com/privacy", "https://", "https://:443/privacy"} {
		cfg := config.DefaultConfig()
		cfg.PrivacyURL = invalid
		if cfg.Validate() == nil {
			t.Fatalf("privacy URL %q should be invalid", invalid)
		}
	}
}

func TestValidateServerName(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServerName = ""
	if cfg.Validate() == nil {
		t.Fatal("empty server name should fail validation")
	}

	cfg = config.DefaultConfig()
	cfg.ServerName = string(make([]byte, 201))
	if cfg.Validate() == nil {
		t.Fatal("overlong server name should fail validation")
	}
}
