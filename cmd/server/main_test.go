package server

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

const (
	envPublicHost       = "env.example.com"
	flagPublicHost      = "flag.example.com"
	duration120s        = "120s"
	allowedOriginsValue = "https://a.example.com, https://b.example.com"
	perfStatsInterval5s = "5s"
	maxDuration2m       = "2m0s"
	parseFlagsFmt       = "parse flags: %v"
)

func TestApplyServerFlagOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.PublicHost = envPublicHost
	cfg.MaxTestDuration = 5 * time.Minute

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--public-host=" + flagPublicHost,
		"--server-name=Frankfurt 10G",
		"--max-test-duration=" + duration120s,
		"--allowed-origins=" + allowedOriginsValue,
	}); err != nil {
		t.Fatalf(parseFlagsFmt, err)
	}

	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}

	if cfg.PublicHost != flagPublicHost {
		t.Fatalf("public host = %q, want %q", cfg.PublicHost, flagPublicHost)
	}
	if cfg.ServerName != "Frankfurt 10G" {
		t.Fatalf("server name = %q, want Frankfurt 10G", cfg.ServerName)
	}
	if cfg.MaxTestDuration.String() != maxDuration2m {
		t.Fatalf("max test duration = %s, want %s", cfg.MaxTestDuration, maxDuration2m)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://a.example.com" || cfg.AllowedOrigins[1] != "https://b.example.com" {
		t.Fatalf("allowed origins = %#v, want two trimmed entries", cfg.AllowedOrigins)
	}
}

func TestApplyServerFlagOverridesInvalidDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{"--max-test-duration=not-a-duration"}); err != nil {
		t.Fatalf(parseFlagsFmt, err)
	}

	if applyServerFlagOverrides(cfg, fs, fv) == nil {
		t.Fatal("expected error for invalid max-test-duration")
	}
}

func TestApplyServerFlagOverridesFailsFastOnInvalidDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.PerfStatsInterval = 30 * time.Second

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--max-test-duration=not-a-duration",
		"--perf-stats-interval=" + perfStatsInterval5s,
	}); err != nil {
		t.Fatalf(parseFlagsFmt, err)
	}

	if applyServerFlagOverrides(cfg, fs, fv) == nil {
		t.Fatal("expected error for invalid duration")
	}
	if cfg.PerfStatsInterval != 30*time.Second {
		t.Fatalf("perf stats interval changed despite earlier duration parse error: got %s", cfg.PerfStatsInterval)
	}
}

func TestCapacityGbpsFlagBypassValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{"--capacity-gbps=0"}); err != nil {
		t.Fatalf(parseFlagsFmt, err)
	}
	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}
	if cfg.Validate() == nil {
		t.Fatal("expected validation error for capacity-gbps=0")
	}
}
