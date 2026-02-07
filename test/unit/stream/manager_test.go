package stream_test

import (
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func newTestManager() *stream.Manager {
	m := stream.NewManager(10, 5)
	m.Start()
	return m
}

func testConfig(dir types.Direction) types.StreamConfig {
	return types.StreamConfig{
		Protocol:  types.ProtocolTCP,
		Direction: dir,
		Duration:  10 * time.Second,
		Streams:   2,
		ClientIP:  "10.0.0.1",
	}
}

func TestManagerCreateStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, err := m.CreateStream(testConfig(types.DirectionDownload))
	if err != nil {
		t.Fatalf("CreateStream: %v", err)
	}
	if state.Config.ID == "" {
		t.Fatal("stream ID should be auto-generated")
	}
	if state.Status != types.StreamStatusPending {
		t.Errorf("status = %v, want Pending", state.Status)
	}
	if m.ActiveCount() != 1 {
		t.Errorf("active = %d, want 1", m.ActiveCount())
	}
}

func TestManagerStartStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	if err := m.StartStream(state.Config.ID); err != nil {
		t.Fatalf("StartStream: %v", err)
	}
}

func TestManagerStartStreamNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	if err := m.StartStream("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent stream")
	}
}

func TestManagerGetStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionUpload))
	got, err := m.GetStream(state.Config.ID)
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	if got.Config.Direction != types.DirectionUpload {
		t.Errorf("direction = %v, want upload", got.Config.Direction)
	}
}

func TestManagerGetStreamNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	_, err := m.GetStream("missing")
	if err == nil {
		t.Fatal("expected error for missing stream")
	}
}

func TestManagerCancelStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	m.StartStream(state.Config.ID)

	if err := m.CancelStream(state.Config.ID); err != nil {
		t.Fatalf("CancelStream: %v", err)
	}

	got, _ := m.GetStream(state.Config.ID)
	snap := got.GetState()
	if snap.Status != types.StreamStatusCancelled {
		t.Errorf("status = %v, want Cancelled", snap.Status)
	}
	if m.ActiveCount() != 0 {
		t.Errorf("active = %d, want 0 after cancel", m.ActiveCount())
	}
}

func TestManagerCancelStreamNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	if err := m.CancelStream("missing"); err == nil {
		t.Fatal("expected error for missing stream")
	}
}

func TestManagerCompleteStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	m.StartStream(state.Config.ID)

	metrics := types.Metrics{ThroughputMbps: 500.0}
	if err := m.CompleteStream(state.Config.ID, metrics); err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}

	got, _ := m.GetStream(state.Config.ID)
	snap := got.GetState()
	if snap.Status != types.StreamStatusCompleted {
		t.Errorf("status = %v, want Completed", snap.Status)
	}
	if snap.Metrics.ThroughputMbps != 500.0 {
		t.Errorf("throughput = %v, want 500", snap.Metrics.ThroughputMbps)
	}
	if m.ActiveCount() != 0 {
		t.Errorf("active = %d, want 0 after complete", m.ActiveCount())
	}
}

func TestManagerCompleteStreamNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	err := m.CompleteStream("missing", types.Metrics{})
	if err == nil {
		t.Fatal("expected error for missing stream")
	}
}

func TestManagerFailStream(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionUpload))
	m.StartStream(state.Config.ID)

	if err := m.FailStream(state.Config.ID, types.Metrics{}); err != nil {
		t.Fatalf("FailStream: %v", err)
	}

	got, _ := m.GetStream(state.Config.ID)
	snap := got.GetState()
	if snap.Status != types.StreamStatusFailed {
		t.Errorf("status = %v, want Failed", snap.Status)
	}
	if m.ActiveCount() != 0 {
		t.Errorf("active = %d, want 0 after fail", m.ActiveCount())
	}
}

func TestManagerUpdateMetrics(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	m.StartStream(state.Config.ID)

	metrics := types.Metrics{ThroughputMbps: 250.0, BytesTransferred: 1024}
	if err := m.UpdateMetrics(state.Config.ID, metrics); err != nil {
		t.Fatalf("UpdateMetrics: %v", err)
	}

	got, _ := m.GetStream(state.Config.ID)
	snap := got.GetState()
	if snap.Metrics.ThroughputMbps != 250.0 {
		t.Errorf("throughput = %v, want 250", snap.Metrics.ThroughputMbps)
	}
}

func TestManagerUpdateMetricsNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	if err := m.UpdateMetrics("missing", types.Metrics{}); err == nil {
		t.Fatal("expected error for missing stream")
	}
}

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
	cfg.ClientIP = "10.0.0.1"

	_, _ = m.CreateStream(cfg)
	_, _ = m.CreateStream(cfg)

	_, err := m.CreateStream(cfg)
	if err == nil {
		t.Fatal("expected error when max per-IP streams reached")
	}

	// Different IP should still work
	cfg.ClientIP = "10.0.0.2"
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
	cfg.ID = "fixed-id"

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
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := testConfig(types.DirectionDownload)
			state, err := m.CreateStream(cfg)
			if err != nil {
				return
			}
			m.StartStream(state.Config.ID)
			m.UpdateMetrics(state.Config.ID, types.Metrics{ThroughputMbps: 100})
			m.GetActiveStreams()
			m.CompleteStream(state.Config.ID, types.Metrics{})
		}()
	}
	wg.Wait()

	if m.ActiveCount() != 0 {
		t.Errorf("active = %d after all completed, want 0", m.ActiveCount())
	}
}

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
