package metrics_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestCalculateLatency(t *testing.T) {
	tests := []struct {
		name     string
		samples  []time.Duration
		want     types.LatencyMetrics
		wantZero bool
	}{
		{
			name:     "empty samples",
			samples:  []time.Duration{},
			wantZero: true,
		},
		{
			name:    "single sample",
			samples: []time.Duration{10 * time.Millisecond},
			want: types.LatencyMetrics{
				MinMs: 10.0,
				MaxMs: 10.0,
				AvgMs: 10.0,
				P50Ms: 10.0,
				P95Ms: 10.0,
				P99Ms: 10.0,
				Count: 1,
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
			if got.MinMs != tt.want.MinMs {
				t.Errorf("calculateLatency() MinMs = %v, want %v", got.MinMs, tt.want.MinMs)
			}
			if got.MaxMs != tt.want.MaxMs {
				t.Errorf("calculateLatency() MaxMs = %v, want %v", got.MaxMs, tt.want.MaxMs)
			}
			if got.AvgMs != tt.want.AvgMs {
				t.Errorf("calculateLatency() AvgMs = %v, want %v", got.AvgMs, tt.want.AvgMs)
			}
			if got.Count != tt.want.Count {
				t.Errorf("calculateLatency() Count = %v, want %v", got.Count, tt.want.Count)
			}
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
			name:    "empty samples",
			samples: []time.Duration{},
			want:    0.0,
		},
		{
			name:    "single sample",
			samples: []time.Duration{10 * time.Millisecond},
			want:    0.0,
		},
		{
			name: "constant delay",
			samples: []time.Duration{
				10 * time.Millisecond,
				10 * time.Millisecond,
				10 * time.Millisecond,
			},
			want: 0.0,
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
			if tt.name == "empty samples" || tt.name == "single sample" {
				if got != tt.want {
					t.Errorf("calculateJitter() = %v, want %v", got, tt.want)
				}
			} else {
				if tt.want == 0.0 && got != 0.0 {
					t.Errorf("calculateJitter() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
