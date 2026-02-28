package metrics_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	emptySamplesName  = "empty samples"
	singleSampleName  = "single sample"
	zeroFloat64       = 0.0
	singleSampleCount = 1
)

func assertLatencyMetrics(t *testing.T, got, want types.LatencyMetrics) {
	t.Helper()
	if got.MinMs != want.MinMs {
		t.Errorf("calculateLatency() MinMs = %v, want %v", got.MinMs, want.MinMs)
	}
	if got.MaxMs != want.MaxMs {
		t.Errorf("calculateLatency() MaxMs = %v, want %v", got.MaxMs, want.MaxMs)
	}
	if got.AvgMs != want.AvgMs {
		t.Errorf("calculateLatency() AvgMs = %v, want %v", got.AvgMs, want.AvgMs)
	}
	if got.Count != want.Count {
		t.Errorf("calculateLatency() Count = %v, want %v", got.Count, want.Count)
	}
}

func TestCalculateLatency(t *testing.T) {
	tests := []struct {
		name     string
		samples  []time.Duration
		want     types.LatencyMetrics
		wantZero bool
	}{
		{
			name:     emptySamplesName,
			samples:  []time.Duration{},
			wantZero: true,
		},
		{
			name:    singleSampleName,
			samples: []time.Duration{10 * time.Millisecond},
			want: types.LatencyMetrics{
				MinMs: 10.0,
				MaxMs: 10.0,
				AvgMs: 10.0,
				P50Ms: 10.0,
				P95Ms: 10.0,
				P99Ms: 10.0,
				Count: singleSampleCount,
			},
		},
		{
			name: "multiple samples",
			samples: []time.Duration{
				1 * time.Millisecond,
				2 * time.Millisecond,
				3 * time.Millisecond,
				4 * time.Millisecond,
				5 * time.Millisecond,
			},
			want: types.LatencyMetrics{
				MinMs: 1.0,
				MaxMs: 5.0,
				AvgMs: 3.0,
				P50Ms: 3.0,
				P95Ms: 5.0,
				P99Ms: 5.0,
				Count: 5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metrics.CalculateLatency(tt.samples)
			if tt.wantZero {
				if got.Count != 0 {
					t.Errorf("calculateLatency() = %v, want zero count", got)
				}
				return
			}
			assertLatencyMetrics(t, got, tt.want)
		})
	}
}

func TestCalculateJitter(t *testing.T) {
	tests := []struct {
		name    string
		samples []time.Duration
		want    float64
	}{
		{
			name:    emptySamplesName,
			samples: []time.Duration{},
			want:    zeroFloat64,
		},
		{
			name:    singleSampleName,
			samples: []time.Duration{10 * time.Millisecond},
			want:    zeroFloat64,
		},
		{
			name: "constant delay",
			samples: []time.Duration{
				10 * time.Millisecond,
				10 * time.Millisecond,
				10 * time.Millisecond,
			},
			want: zeroFloat64,
		},
		{
			name: "varying delay",
			samples: []time.Duration{
				10 * time.Millisecond,
				12 * time.Millisecond,
				8 * time.Millisecond,
			},
			want: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metrics.CalculateJitter(tt.samples)
			if tt.name == emptySamplesName || tt.name == singleSampleName {
				if got != tt.want {
					t.Errorf("calculateJitter() = %v, want %v", got, tt.want)
				}
				return
			}
			if tt.want == zeroFloat64 && got != zeroFloat64 {
				t.Errorf("calculateJitter() = %v, want %v", got, tt.want)
			}
		})
	}
}
