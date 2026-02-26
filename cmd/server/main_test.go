package server

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	envServerName       = "Env Name"
	flagServerName      = "Flag Name"
	duration120s        = "120s"
	allowedOriginsValue = "https://a.example.com, https://b.example.com"
	registryInterval5s  = "5s"
	loopbackClientIP    = "127.0.0.1"
	maxDuration2m       = "2m0s"
)

func TestApplyServerFlagOverrides(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServerName = envServerName
	cfg.MaxTestDuration = 5 * time.Minute

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--server-name=" + flagServerName,
		"--max-test-duration=" + duration120s,
		"--allowed-origins=" + allowedOriginsValue,
		"--registry-enabled=true",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}

	if cfg.ServerName != flagServerName {
		t.Fatalf("server name = %q, want %q", cfg.ServerName, flagServerName)
	}
	if cfg.MaxTestDuration.String() != maxDuration2m {
		t.Fatalf("max test duration = %s, want %s", cfg.MaxTestDuration, maxDuration2m)
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

	if applyServerFlagOverrides(cfg, fs, fv) == nil {
		t.Fatal("expected error for invalid max-test-duration")
	}
}

func TestApplyServerFlagOverridesFailsFastOnInvalidDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RegistryInterval = 30 * time.Second

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--max-test-duration=not-a-duration",
		"--registry-interval=" + registryInterval5s,
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if applyServerFlagOverrides(cfg, fs, fv) == nil {
		t.Fatal("expected error for invalid duration")
	}
	if cfg.RegistryInterval != 30*time.Second {
		t.Fatalf("registry interval changed despite earlier duration parse error: got %s", cfg.RegistryInterval)
	}
}

func TestRunShutdownBroadcastDrain(t *testing.T) {
	manager := stream.NewManager(10, 10)
	manager.Start()
	wsServer := websocket.NewServer()

	done := make(chan struct{})
	go func() {
		broadcastMetrics(manager, wsServer)
		close(done)
	}()

	state, err := manager.CreateStream(types.StreamConfig{
		Protocol:   types.ProtocolTCP,
		Direction:  types.DirectionDownload,
		Duration:   2 * time.Second,
		Streams:    1,
		PacketSize: 1400,
		StartTime:  time.Now(),
		ClientIP:   loopbackClientIP,
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if err := manager.StartStream(state.Config.ID); err != nil {
		t.Fatalf("start stream: %v", err)
	}
	if err := manager.UpdateMetrics(state.Config.ID, types.Metrics{ThroughputMbps: 42}); err != nil {
		t.Fatalf("update metrics: %v", err)
	}

	manager.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcastMetrics goroutine did not drain after manager.Stop")
	}
}

func TestCapacityGbpsFlagBypassValidation(t *testing.T) {
	cfg := config.DefaultConfig()
	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{"--capacity-gbps=0"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}
	if cfg.Validate() == nil {
		t.Fatal("expected validation error for capacity-gbps=0")
	}
}
