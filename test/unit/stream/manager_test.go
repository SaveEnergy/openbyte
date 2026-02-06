package stream_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestPendingStreamCleanup(t *testing.T) {
	manager := stream.NewManager(10, 10)
	manager.SetRetentionPeriod(1 * time.Second)
	manager.Start()
	defer manager.Stop()

	// Create a stream but never call StartStream — stays Pending
	cfg := types.StreamConfig{
		Protocol:  types.ProtocolTCP,
		Direction: types.DirectionDownload,
		Duration:  30 * time.Second,
		Streams:   1,
		StartTime: time.Now().Add(-60 * time.Second), // Created 60s ago
	}
	state, err := manager.CreateStream(cfg)
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	streamID := state.Config.ID

	// Verify stream exists and is Pending
	s, err := manager.GetStream(streamID)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	snap := s.GetState()
	if snap.Status != types.StreamStatusPending {
		t.Fatalf("status = %v, want %v", snap.Status, types.StreamStatusPending)
	}

	// Verify it counts as active
	if c := manager.ActiveCount(); c != 1 {
		t.Fatalf("ActiveCount = %d, want 1", c)
	}

	// Wait for cleanup cycle (runs every 10s but we can't speed that up
	// without exposing internals, so we test the cleanup method effect
	// by waiting for it)
	// Instead, create another stream to verify max slot consumption
	// and just verify the pending stream was there
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_, getErr := manager.GetStream(streamID)
		if getErr != nil {
			// Stream was cleaned up
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("pending stream %s was not cleaned up within 15s", streamID)
}

func TestActiveCountMatchesActiveStreams(t *testing.T) {
	manager := stream.NewManager(10, 10)
	manager.Start()
	defer manager.Stop()

	if c := manager.ActiveCount(); c != 0 {
		t.Fatalf("initial ActiveCount = %d, want 0", c)
	}

	cfg := types.StreamConfig{
		Protocol:  types.ProtocolTCP,
		Direction: types.DirectionDownload,
		Duration:  30 * time.Second,
		Streams:   1,
	}

	state, err := manager.CreateStream(cfg)
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	// Pending counts as active
	if c := manager.ActiveCount(); c != 1 {
		t.Fatalf("after create: ActiveCount = %d, want 1", c)
	}

	// Start stream
	if err := manager.StartStream(state.Config.ID); err != nil {
		t.Fatalf("StartStream: %v", err)
	}

	if c := manager.ActiveCount(); c != 1 {
		t.Fatalf("after start: ActiveCount = %d, want 1", c)
	}

	// Complete stream
	if err := manager.CompleteStream(state.Config.ID, types.Metrics{}); err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}

	if c := manager.ActiveCount(); c != 0 {
		t.Fatalf("after complete: ActiveCount = %d, want 0", c)
	}
}
