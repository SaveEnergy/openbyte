package types

import (
	"math"
	"net"
	"sync"
	"time"
)

type RTTCollector struct {
	samples    []float64
	head       int
	count      int
	baseline   float64
	mu         sync.RWMutex
	maxSamples int
}

func NewRTTCollector(maxSamples int) *RTTCollector {
	if maxSamples <= 0 {
		maxSamples = 100
	}
	return &RTTCollector{
		samples:    make([]float64, maxSamples),
		maxSamples: maxSamples,
	}
}

func (r *RTTCollector) AddSample(rttMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.samples[r.head] = rttMs
	r.head = (r.head + 1) % r.maxSamples
	if r.count < r.maxSamples {
		r.count++
	}
}

// activeSamples returns a snapshot of current samples in insertion order.
func (r *RTTCollector) activeSamples() []float64 {
	if r.count == 0 {
		return nil
	}
	out := make([]float64, r.count)
	if r.count < r.maxSamples {
		copy(out, r.samples[:r.count])
	} else {
		// Ring is full: oldest is at r.head, wrap around
		n := copy(out, r.samples[r.head:])
		copy(out[n:], r.samples[:r.head])
	}
	return out
}

func (r *RTTCollector) SetBaseline(rttMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseline = rttMs
}

func (r *RTTCollector) GetMetrics() RTTMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return RTTMetrics{BaselineMs: r.baseline}
	}

	active := r.activeSamples()

	var sum, minVal, maxVal float64
	minVal = active[0]
	maxVal = active[0]

	for _, v := range active {
		sum += v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	avg := sum / float64(len(active))

	var varianceSum float64
	for _, v := range active {
		diff := v - avg
		varianceSum += diff * diff
	}
	jitter := math.Sqrt(varianceSum / float64(len(active)))

	current := active[len(active)-1]

	return RTTMetrics{
		BaselineMs: r.baseline,
		CurrentMs:  current,
		MinMs:      minVal,
		MaxMs:      maxVal,
		AvgMs:      avg,
		JitterMs:   jitter,
		Samples:    len(active),
	}
}

func measureRTT(addr string, timeout time.Duration) (float64, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	rtt := time.Since(start).Seconds() * 1000
	return rtt, nil
}

func MeasureBaselineRTT(addr string, samples int, timeout time.Duration) (float64, error) {
	if samples <= 0 {
		samples = 10
	}

	var total float64
	var count int

	for i := 0; i < samples; i++ {
		rtt, err := measureRTT(addr, timeout)
		if err != nil {
			continue
		}
		total += rtt
		count++
		time.Sleep(50 * time.Millisecond)
	}

	if count == 0 {
		return 0, net.UnknownNetworkError("no successful RTT samples")
	}

	return total / float64(count), nil
}
