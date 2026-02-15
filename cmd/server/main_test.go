package server

import (
	"time"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
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

func TestApplyServerFlagOverridesFailsFastOnInvalidDuration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RegistryInterval = 30 * time.Second

	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse([]string{
		"--max-test-duration=not-a-duration",
		"--registry-interval=5s",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := applyServerFlagOverrides(cfg, fs, fv); err == nil {
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
		ClientIP:   "127.0.0.1",
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
