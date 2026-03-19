package stream_test

import (
	"sync"
	"testing"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestManagerMaxConcurrentStreams(t *testing.T) {
	m := stream.NewManager(2, 0)
	m.Start()
	defer m.Stop()

	_, err1 := m.CreateStream(testConfig(types.DirectionDownload))
	if err1 != nil {
		t.Fatalf("first stream: %v", err1)
	}
	_, err2 := m.CreateStream(testConfig(types.DirectionDownload))
	if err2 != nil {
		t.Fatalf("second stream: %v", err2)
	}

	_, err3 := m.CreateStream(testConfig(types.DirectionDownload))
	if err3 == nil {
		t.Fatal("expected error when max concurrent streams reached")
	}
}

func TestManagerMaxStreamsPerIP(t *testing.T) {
	m := stream.NewManager(100, 2)
	m.Start()
	defer m.Stop()

	cfg := testConfig(types.DirectionDownload)
	cfg.ClientIP = testIPPrimary

	_, _ = m.CreateStream(cfg)
	_, _ = m.CreateStream(cfg)

	_, err := m.CreateStream(cfg)
	if err == nil {
		t.Fatal("expected error when max per-IP streams reached")
	}

	// Different IP should still work
	cfg.ClientIP = testIPSecondary
	_, err = m.CreateStream(cfg)
	if err != nil {
		t.Fatalf("different IP should succeed: %v", err)
	}
}

func TestManagerGetActiveStreams(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	s1, _ := m.CreateStream(testConfig(types.DirectionDownload))
	m.StartStream(s1.Config.ID)

	s2, _ := m.CreateStream(testConfig(types.DirectionUpload))
	m.StartStream(s2.Config.ID)

	// Complete s1
	m.CompleteStream(s1.Config.ID, types.Metrics{})

	active := m.GetActiveStreams()
	if len(active) != 1 {
		t.Fatalf("active = %d, want 1", len(active))
	}
	if active[0] != s2.Config.ID {
		t.Errorf("active stream = %q, want %q", active[0], s2.Config.ID)
	}
}

func TestManagerDuplicateStreamID(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	cfg := testConfig(types.DirectionDownload)
	cfg.ID = streamFixedID

	_, err := m.CreateStream(cfg)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = m.CreateStream(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate stream ID")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	m := stream.NewManager(100, 0)
	m.Start()
	defer m.Stop()

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			cfg := testConfig(types.DirectionDownload)
			state, err := m.CreateStream(cfg)
			if err != nil {
				return
			}
			m.StartStream(state.Config.ID)
			m.UpdateMetrics(state.Config.ID, types.Metrics{ThroughputMbps: 100})
			m.GetActiveStreams()
			m.CompleteStream(state.Config.ID, types.Metrics{})
		})
	}
	wg.Wait()

	if m.ActiveCount() != 0 {
		t.Errorf("active = %d after all completed, want 0", m.ActiveCount())
	}
}
