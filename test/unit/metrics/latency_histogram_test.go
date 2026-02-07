package metrics_test

import (
	"sync"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func TestHistogramRecordAndCopy(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 100)

	h.Record(5 * time.Millisecond)
	h.Record(10 * time.Millisecond)
	h.Record(10 * time.Millisecond)

	buckets := make([]uint32, h.BucketCount())
	overflow := h.CopyTo(buckets)

	if overflow != 0 {
		t.Errorf("overflow = %d, want 0", overflow)
	}
	if buckets[5] != 1 {
		t.Errorf("bucket[5] = %d, want 1", buckets[5])
	}
	if buckets[10] != 2 {
		t.Errorf("bucket[10] = %d, want 2", buckets[10])
	}
}

func TestHistogramOverflow(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 50)

	h.Record(100 * time.Millisecond) // beyond 50 buckets
	h.Record(200 * time.Millisecond)

	buckets := make([]uint32, h.BucketCount())
	overflow := h.CopyTo(buckets)

	if overflow != 2 {
		t.Errorf("overflow = %d, want 2", overflow)
	}
}

func TestHistogramNegativeSample(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 100)

	h.Record(-5 * time.Millisecond) // should clamp to bucket 0

	buckets := make([]uint32, h.BucketCount())
	h.CopyTo(buckets)

	if buckets[0] != 1 {
		t.Errorf("bucket[0] = %d, want 1 for negative sample", buckets[0])
	}
}

func TestHistogramReset(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 100)

	h.Record(5 * time.Millisecond)
	h.Record(500 * time.Millisecond) // overflow
	h.Reset()

	buckets := make([]uint32, h.BucketCount())
	overflow := h.CopyTo(buckets)

	if overflow != 0 {
		t.Errorf("overflow after reset = %d, want 0", overflow)
	}
	for i, v := range buckets {
		if v != 0 {
			t.Errorf("bucket[%d] = %d after reset, want 0", i, v)
			break
		}
	}
}

func TestHistogramBucketWidth(t *testing.T) {
	h := metrics.NewLatencyHistogram(2*time.Millisecond, 50)
	if h.BucketWidth() != 2*time.Millisecond {
		t.Errorf("BucketWidth = %v, want 2ms", h.BucketWidth())
	}
}

func TestHistogramBucketCount(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 200)
	if h.BucketCount() != 200 {
		t.Errorf("BucketCount = %d, want 200", h.BucketCount())
	}
}

func TestHistogramDefaultBucketWidth(t *testing.T) {
	h := metrics.NewLatencyHistogram(0, 10)
	if h.BucketWidth() != time.Millisecond {
		t.Errorf("default BucketWidth = %v, want 1ms", h.BucketWidth())
	}
}

func TestHistogramDefaultBucketCount(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 0)
	if h.BucketCount() != 1 {
		t.Errorf("default BucketCount = %d, want 1", h.BucketCount())
	}
}

func TestHistogramZeroSample(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 100)
	h.Record(0)

	buckets := make([]uint32, h.BucketCount())
	h.CopyTo(buckets)

	if buckets[0] != 1 {
		t.Errorf("bucket[0] = %d, want 1 for zero sample", buckets[0])
	}
}

func TestHistogramExactBoundary(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 10)

	// Exactly at the last bucket boundary
	h.Record(9 * time.Millisecond)

	buckets := make([]uint32, h.BucketCount())
	h.CopyTo(buckets)

	if buckets[9] != 1 {
		t.Errorf("bucket[9] = %d, want 1", buckets[9])
	}

	// One above → overflow
	h.Record(10 * time.Millisecond)
	overflow := h.CopyTo(buckets)
	if overflow != 1 {
		t.Errorf("boundary overflow = %d, want 1", overflow)
	}
}

func TestHistogramConcurrent(t *testing.T) {
	h := metrics.NewLatencyHistogram(time.Millisecond, 100)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(ms int) {
			defer wg.Done()
			h.Record(time.Duration(ms%50) * time.Millisecond)
		}(i)
	}
	wg.Wait()

	buckets := make([]uint32, h.BucketCount())
	overflow := h.CopyTo(buckets)

	var total uint32
	for _, v := range buckets {
		total += v
	}
	total += overflow

	if total != 100 {
		t.Errorf("total samples = %d, want 100", total)
	}
}
