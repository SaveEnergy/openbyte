package stream

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func TestCleanupRemovesExpiredPendingStream(t *testing.T) {
	m := NewManager(10, 0)

	cfg := types.StreamConfig{
		ID:        "pending-expired",
		Protocol:  types.ProtocolTCP,
		Direction: types.DirectionDownload,
		Duration:  5 * time.Second,
		Streams:   1,
		StartTime: time.Now().Add(-31 * time.Second),
	}
	_, err := m.CreateStream(cfg)
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if got := m.ActiveCount(); got != 1 {
		t.Fatalf("active count before cleanup = %d, want 1", got)
	}

	m.cleanup()

	if _, err := m.GetStream(cfg.ID); err == nil {
		t.Fatalf("expected stream %q to be removed by cleanup", cfg.ID)
	}
	if got := m.ActiveCount(); got != 0 {
		t.Fatalf("active count after cleanup = %d, want 0", got)
	}
}
