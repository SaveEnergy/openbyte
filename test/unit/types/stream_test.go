package types_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	exampleHost             = "example.com"
	localhostHost           = "localhost"
	loopbackIPv4Host        = "127.0.0.1"
	loopbackIPv6Host        = "::1"
	streamIDFixture         = "stream-123"
	httpExampleURL          = "https://example.com"
	httpsExamplePorted      = "https://example.com:8443"
	streamDuration          = 30 * time.Second
	metricsStartDelta       = -10 * time.Second
	throughputTarget        = 1000.0
	statusWantFmt           = "Status = %v, want %v"
	stripHostPortFmt        = "StripHostPort(%q) = %q, want %q"
	originHostFmt           = "OriginHost(%q) = %q, want %q"
	streamStartTimeErr      = "StartTime should be set when status changes to running"
	streamEndTimeErr        = "EndTime should be set when status changes to completed"
	streamMetricsWantFmt    = "Metrics.ThroughputMbps = %v, want %v"
	streamProgressRangeFmt  = "Progress = %v, should be between 0 and 100"
	streamSnapshotIDFmt     = "Snapshot.Config.ID = %v, want %s"
	streamSnapshotStatusFmt = "Snapshot.Status = %v, want %v"
)

func TestStripHostPort(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{exampleHost, exampleHost},
		{exampleHost + ":8080", exampleHost},
		{"[" + loopbackIPv6Host + "]:8080", loopbackIPv6Host},
		{"[" + loopbackIPv6Host + "]", loopbackIPv6Host},
		{loopbackIPv4Host + ":3000", loopbackIPv4Host},
		{loopbackIPv4Host, loopbackIPv4Host},
	}
	for _, tc := range tests {
		got := types.StripHostPort(tc.input)
		if got != tc.want {
			t.Errorf(stripHostPortFmt, tc.input, got, tc.want)
		}
	}
}

func TestOriginHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{httpExampleURL, exampleHost},
		{httpsExamplePorted, exampleHost},
		{"http://" + localhostHost + ":3000", localhostHost},
		{"http://[" + loopbackIPv6Host + "]:8080", loopbackIPv6Host},
		{exampleHost, exampleHost},
		{exampleHost + ":8080", exampleHost},
		{"", ""},
	}
	for _, tc := range tests {
		got := types.OriginHost(tc.input)
		if got != tc.want {
			t.Errorf(originHostFmt, tc.input, got, tc.want)
		}
	}
}

func TestStreamStateUpdateStatus(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			Duration: streamDuration,
		},
		Status: types.StreamStatusPending,
	}

	state.UpdateStatus(types.StreamStatusStarting)
	if state.Status != types.StreamStatusStarting {
		t.Errorf(statusWantFmt, state.Status, types.StreamStatusStarting)
	}

	state.UpdateStatus(types.StreamStatusRunning)
	if state.Status != types.StreamStatusRunning {
		t.Errorf(statusWantFmt, state.Status, types.StreamStatusRunning)
	}
	if state.StartTime.IsZero() {
		t.Error(streamStartTimeErr)
	}

	state.UpdateStatus(types.StreamStatusCompleted)
	if state.Status != types.StreamStatusCompleted {
		t.Errorf(statusWantFmt, state.Status, types.StreamStatusCompleted)
	}
	if state.EndTime.IsZero() {
		t.Error(streamEndTimeErr)
	}
}

func TestStreamStateUpdateMetrics(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			Duration: streamDuration,
		},
		Status:    types.StreamStatusRunning,
		StartTime: time.Now().Add(metricsStartDelta),
	}

	m := types.Metrics{
		ThroughputMbps: throughputTarget,
		Timestamp:      time.Now(),
	}

	state.UpdateMetrics(m)

	if state.Metrics.ThroughputMbps != m.ThroughputMbps {
		t.Errorf(streamMetricsWantFmt, state.Metrics.ThroughputMbps, m.ThroughputMbps)
	}

	if state.Progress < 0 || state.Progress > 100 {
		t.Errorf(streamProgressRangeFmt, state.Progress)
	}
}

func TestStreamStateGetState(t *testing.T) {
	state := &types.StreamState{
		Config: types.StreamConfig{
			ID:       streamIDFixture,
			Protocol: types.ProtocolTCP,
		},
		Status: types.StreamStatusRunning,
	}

	snapshot := state.GetState()

	if snapshot.Config.ID != streamIDFixture {
		t.Errorf(streamSnapshotIDFmt, snapshot.Config.ID, streamIDFixture)
	}
	if snapshot.Status != types.StreamStatusRunning {
		t.Errorf(streamSnapshotStatusFmt, snapshot.Status, types.StreamStatusRunning)
	}
}
