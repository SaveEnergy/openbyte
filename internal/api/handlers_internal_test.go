package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestStartCreatedStreamCleanupFailure(t *testing.T) {
	manager := stream.NewManager(10, 10)
	handler := NewHandler(manager)

	cfg := types.StreamConfig{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Protocol:  types.ProtocolTCP,
		Direction: types.DirectionDownload,
		Duration:  10 * time.Second,
		Streams:   1,
		StartTime: time.Now(),
	}
	_, err := manager.CreateStream(cfg)
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if err := manager.CompleteStream(cfg.ID, types.Metrics{}); err != nil {
		t.Fatalf("complete stream: %v", err)
	}

	if err := handler.startCreatedStream(cfg.ID); err == nil {
		t.Fatal("expected startCreatedStream error for terminal stream")
	}

	state, err := manager.GetStream(cfg.ID)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	if got := state.GetState().Status; got != types.StreamStatusCompleted {
		t.Fatalf("status = %s, want %s", got, types.StreamStatusCompleted)
	}
}
