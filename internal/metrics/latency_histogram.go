package metrics

import (
	"sync"
	"time"
)

type LatencyHistogram struct {
	bucketWidth time.Duration
	buckets     []uint32
	overflow    uint32
	mu          sync.RWMutex
}

func NewLatencyHistogram(bucketWidth time.Duration, bucketCount int) *LatencyHistogram {
	if bucketWidth <= 0 {
		bucketWidth = time.Millisecond
	}
	if bucketCount <= 0 {
		bucketCount = 1
	}
	return &LatencyHistogram{
		bucketWidth: bucketWidth,
		buckets:     make([]uint32, bucketCount),
	}
}

func (h *LatencyHistogram) BucketWidth() time.Duration {
	return h.bucketWidth
}

func (h *LatencyHistogram) BucketCount() int {
	return len(h.buckets)
}

func (h *LatencyHistogram) Record(sample time.Duration) {
	h.mu.Lock()
	if sample < 0 {
		sample = 0
	}
	index := int(sample / h.bucketWidth)
	if index >= len(h.buckets) {
		h.overflow++
	} else {
		h.buckets[index]++
	}
	h.mu.Unlock()
}

func (h *LatencyHistogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.buckets {
		h.buckets[i] = 0
	}
	h.overflow = 0
}

func (h *LatencyHistogram) CopyTo(dst []uint32) uint32 {
	h.mu.RLock()
	copy(dst, h.buckets)
	o := h.overflow
	h.mu.RUnlock()
	return o
}
