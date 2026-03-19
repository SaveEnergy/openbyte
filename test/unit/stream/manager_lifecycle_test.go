package stream_test

import (
	"errors"
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

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
		t.Fatalf(startStreamErrFmt, err)
	}
}

func TestManagerStartStreamNotFound(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	if m.StartStream(streamUnknownID) == nil {
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

	_, err := m.GetStream(streamMissingID)
	if err == nil {
		t.Fatal(expectedErrMissingStream)
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

	if m.CancelStream(streamMissingID) == nil {
		t.Fatal(expectedErrMissingStream)
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

	err := m.CompleteStream(streamMissingID, types.Metrics{})
	if err == nil {
		t.Fatal(expectedErrMissingStream)
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

func TestManagerFailStreamWithErrorStoresReason(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	streamState, _ := m.CreateStream(testConfig(types.DirectionUpload))
	if err := m.StartStream(streamState.Config.ID); err != nil {
		t.Fatalf(startStreamErrFmt, err)
	}

	cause := errors.New("upload stream stalled")
	if err := m.FailStreamWithError(
		streamState.Config.ID,
		types.Metrics{BytesTransferred: 128},
		cause,
	); err != nil {
		t.Fatalf("FailStreamWithError: %v", err)
	}

	got, _ := m.GetStream(streamState.Config.ID)
	snap := got.GetState()
	if snap.Error == nil {
		t.Fatal("stream error = nil, want failure reason")
	}
	if snap.Error.Error() != cause.Error() {
		t.Fatalf("stream error = %q, want %q", snap.Error.Error(), cause.Error())
	}
	if snap.Status != types.StreamStatusFailed {
		t.Fatalf("status = %v, want Failed", snap.Status)
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

	if m.UpdateMetrics(streamMissingID, types.Metrics{}) == nil {
		t.Fatal(expectedErrMissingStream)
	}
}
