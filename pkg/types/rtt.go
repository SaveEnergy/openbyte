package types

import (
	"math"
	"net"
	"sync"
	"time"
)

type RTTCollector struct {
	samples    []float64
	baseline   float64
	mu         sync.RWMutex
	maxSamples int
}

func NewRTTCollector(maxSamples int) *RTTCollector {
	if maxSamples <= 0 {
		maxSamples = 100
	}
	return &RTTCollector{
		samples:    make([]float64, 0, maxSamples),
		maxSamples: maxSamples,
	}
}

func (r *RTTCollector) AddSample(rttMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.samples) >= r.maxSamples {
		r.samples = r.samples[1:]
	}
	r.samples = append(r.samples, rttMs)
}

func (r *RTTCollector) SetBaseline(rttMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.baseline = rttMs
}

func (r *RTTCollector) GetMetrics() RTTMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.samples) == 0 {
		return RTTMetrics{BaselineMs: r.baseline}
	}

	var sum, minVal, maxVal float64
	minVal = r.samples[0]
	maxVal = r.samples[0]

	for _, v := range r.samples {
		sum += v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	avg := sum / float64(len(r.samples))

	var varianceSum float64
	for _, v := range r.samples {
		diff := v - avg
		varianceSum += diff * diff
	}
	jitter := math.Sqrt(varianceSum / float64(len(r.samples)))

	current := r.samples[len(r.samples)-1]

	return RTTMetrics{
		BaselineMs: r.baseline,
		CurrentMs:  current,
		MinMs:      minVal,
		MaxMs:      maxVal,
		AvgMs:      avg,
		JitterMs:   jitter,
		Samples:    len(r.samples),
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
