package metrics

import (
	"math"
	"testing"
)

func TestPercentileHistogramTargetIntegerCeil(t *testing.T) {
	t.Parallel()
	for count := int64(1); count <= 8000; count++ {
		for _, tc := range []struct {
			num, denom int64
			ratio      float64
		}{
			{1, 2, 0.50},
			{95, 100, 0.95},
			{99, 100, 0.99},
		} {
			got := max((count*tc.num+tc.denom-1)/tc.denom, 1)
			want := max(int64(math.Ceil(float64(count)*tc.ratio)), 1)
			if got != want {
				t.Fatalf("count=%d num=%d denom=%d ratio=%v: got target %d want %d", count, tc.num, tc.denom, tc.ratio, got, want)
			}
		}
	}
}
