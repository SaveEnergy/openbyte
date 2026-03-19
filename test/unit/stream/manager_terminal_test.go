package stream_test

import (
	"sync"
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

func TestManagerStartStreamDoesNotOverrideTerminalStatus(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	if err := m.StartStream(state.Config.ID); err != nil {
		t.Fatalf(startStreamErrFmt, err)
	}
	if err := m.CancelStream(state.Config.ID); err != nil {
		t.Fatalf("CancelStream: %v", err)
	}
	if m.StartStream(state.Config.ID) == nil {
		t.Fatal("expected error when starting terminal stream")
	}

	got, _ := m.GetStream(state.Config.ID)
	if got.GetState().Status != types.StreamStatusCancelled {
		t.Fatalf("status = %v, want %v", got.GetState().Status, types.StreamStatusCancelled)
	}
}

func TestManagerUpdateMetricsRejectedAfterTerminal(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	if err := m.StartStream(state.Config.ID); err != nil {
		t.Fatalf(startStreamErrFmt, err)
	}
	if err := m.CompleteStream(state.Config.ID, types.Metrics{ThroughputMbps: 111}); err != nil {
		t.Fatalf("CompleteStream: %v", err)
	}
	if m.UpdateMetrics(state.Config.ID, types.Metrics{ThroughputMbps: 999}) == nil {
		t.Fatal("expected error when updating terminal stream metrics")
	}

	got, _ := m.GetStream(state.Config.ID)
	if got.GetState().Metrics.ThroughputMbps != 111 {
		t.Fatalf("throughput changed after terminal status, got %v want 111", got.GetState().Metrics.ThroughputMbps)
	}
}

func TestManagerTerminalStatusImmutable(t *testing.T) {
	m := newTestManager()
	defer m.Stop()

	state, _ := m.CreateStream(testConfig(types.DirectionDownload))
	if err := m.StartStream(state.Config.ID); err != nil {
		t.Fatalf(startStreamErrFmt, err)
	}

	var wg sync.WaitGroup
	type result struct {
		kind string
		err  error
	}
	results := make(chan result, 3)

	wg.Add(3)
	go func() {
		defer wg.Done()
		results <- result{kind: "cancel", err: m.CancelStream(state.Config.ID)}
	}()
	go func() {
		defer wg.Done()
		results <- result{kind: "complete", err: m.CompleteStream(state.Config.ID, types.Metrics{ThroughputMbps: 1})}
	}()
	go func() {
		defer wg.Done()
		results <- result{kind: "fail", err: m.FailStream(state.Config.ID, types.Metrics{ThroughputMbps: 2})}
	}()
	wg.Wait()
	close(results)

	successes := 0
	successKind := ""
	for r := range results {
		if r.err == nil {
			successes++
			successKind = r.kind
		}
	}
	if successes != 1 {
		t.Fatalf("terminal transitions succeeded = %d, want 1", successes)
	}

	got, _ := m.GetStream(state.Config.ID)
	status := got.GetState().Status
	switch successKind {
	case "cancel":
		if status != types.StreamStatusCancelled {
			t.Fatalf("status = %v, want cancelled", status)
		}
	case "complete":
		if status != types.StreamStatusCompleted {
			t.Fatalf("status = %v, want completed", status)
		}
	case "fail":
		if status != types.StreamStatusFailed {
			t.Fatalf("status = %v, want failed", status)
		}
	default:
		t.Fatalf("unexpected success kind %q", successKind)
	}
}
