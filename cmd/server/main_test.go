package server

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestApplyServerFlagOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServerName = "Env Name"
	cfg.MaxTestDuration = 5 * time.Minute

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--server-name=Flag Name",
		"--max-test-duration=120s",
		"--allowed-origins=https://a.example.com, https://b.example.com",
		"--registry-enabled=true",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}

	if cfg.ServerName != "Flag Name" {
		t.Fatalf("server name = %q, want %q", cfg.ServerName, "Flag Name")
	}
	if cfg.MaxTestDuration.String() != "2m0s" {
		t.Fatalf("max test duration = %s, want 2m0s", cfg.MaxTestDuration)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://a.example.com" || cfg.AllowedOrigins[1] != "https://b.example.com" {
		t.Fatalf("allowed origins = %#v, want two trimmed entries", cfg.AllowedOrigins)
	}
	if !cfg.RegistryEnabled {
		t.Fatal("registry enabled should be true")
	}
}

func TestApplyServerFlagOverridesInvalidDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{"--max-test-duration=not-a-duration"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := applyServerFlagOverrides(cfg, fs, fv); err == nil {
		t.Fatal("expected error for invalid max-test-duration")
	}
}
