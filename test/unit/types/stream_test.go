package types_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func TestStreamState_UpdateStatus(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			Duration: 30 * time.Second,
		},
		Status: types.StreamStatusPending,
	}

	state.UpdateStatus(types.StreamStatusStarting)
	if state.Status != types.StreamStatusStarting {
		t.Errorf("Status = %v, want %v", state.Status, types.StreamStatusStarting)
	}

	state.UpdateStatus(types.StreamStatusRunning)
	if state.Status != types.StreamStatusRunning {
		t.Errorf("Status = %v, want %v", state.Status, types.StreamStatusRunning)
	}
	if state.StartTime.IsZero() {
		t.Error("StartTime should be set when status changes to running")
	}

	state.UpdateStatus(types.StreamStatusCompleted)
	if state.Status != types.StreamStatusCompleted {
		t.Errorf("Status = %v, want %v", state.Status, types.StreamStatusCompleted)
	}
	if state.EndTime.IsZero() {
		t.Error("EndTime should be set when status changes to completed")
	}
}

func TestStreamState_UpdateMetrics(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			Duration: 30 * time.Second,
		},
		Status:    types.StreamStatusRunning,
		StartTime: time.Now().Add(-10 * time.Second),
	}

	m := types.Metrics{
		ThroughputMbps: 1000.0,
		Timestamp:      time.Now(),
	}

	state.UpdateMetrics(m)

	if state.Metrics.ThroughputMbps != m.ThroughputMbps {
		t.Errorf("Metrics.ThroughputMbps = %v, want %v", state.Metrics.ThroughputMbps, m.ThroughputMbps)
	}

	if state.Progress < 0 || state.Progress > 100 {
		t.Errorf("Progress = %v, should be between 0 and 100", state.Progress)
	}
}

func TestStreamState_GetState(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			ID:       "stream-123",
			Protocol: types.ProtocolTCP,
		},
		Status: types.StreamStatusRunning,
	}

	snapshot := state.GetState()

	if snapshot.Config.ID != "stream-123" {
		t.Errorf("Snapshot.Config.ID = %v, want stream-123", snapshot.Config.ID)
	}
	if snapshot.Status != types.StreamStatusRunning {
		t.Errorf("Snapshot.Status = %v, want %v", snapshot.Status, types.StreamStatusRunning)
	}
}
