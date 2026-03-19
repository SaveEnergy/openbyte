package stream_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestManagerMetricsChannel(t *testing.T) {
	m := stream.NewManager(10, 0)
	m.SetMetricsUpdateInterval(50 * time.Millisecond)
	m.Start()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	m.StartStream(state.Config.ID)

	ch := m.GetMetricsUpdateChannel()

	select {
	case update := <-ch:
		if update.StreamID != state.Config.ID {
			t.Errorf("update stream = %q, want %q", update.StreamID, state.Config.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no metrics update received within timeout")
	}

	m.CompleteStream(state.Config.ID, types.Metrics{})
	m.Stop()
}

func TestManagerMetricsChannelDropAccounting(t *testing.T) {
	m := stream.NewManager(300, 0)
	m.SetMetricsUpdateInterval(10 * time.Millisecond)
	m.Start()
	defer m.Stop()

	for i := range 150 {
		cfg := testConfig(types.DirectionDownload)
		cfg.ClientIP = testIPPrimary
		state, err := m.CreateStream(cfg)
		if err != nil {
			t.Fatalf("create stream %d: %v", i, err)
		}
		if err := m.StartStream(state.Config.ID); err != nil {
			t.Fatalf("start stream %d: %v", i, err)
		}
	}

	time.Sleep(120 * time.Millisecond)
	if m.DroppedMetricsUpdates() == 0 {
		t.Fatal("expected dropped metrics updates to be accounted")
	}
}
