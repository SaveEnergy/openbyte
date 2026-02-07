package types_test

import (
	"math"
	"sync"
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

func TestRTTCollectorEmpty(t *testing.T) {
	c := types.NewRTTCollector(100)
	m := c.GetMetrics()

	if m.Samples != 0 {
		t.Errorf("samples = %d, want 0", m.Samples)
	}
	if m.AvgMs != 0 {
		t.Errorf("avg = %v, want 0", m.AvgMs)
	}
}

func TestRTTCollectorAddSample(t *testing.T) {
	c := types.NewRTTCollector(100)
	c.AddSample(10.0)
	c.AddSample(20.0)
	c.AddSample(30.0)

	m := c.GetMetrics()
	if m.Samples != 3 {
		t.Errorf("samples = %d, want 3", m.Samples)
	}
	if m.MinMs != 10.0 {
		t.Errorf("min = %v, want 10", m.MinMs)
	}
	if m.MaxMs != 30.0 {
		t.Errorf("max = %v, want 30", m.MaxMs)
	}
	if m.AvgMs != 20.0 {
		t.Errorf("avg = %v, want 20", m.AvgMs)
	}
	if m.CurrentMs != 30.0 {
		t.Errorf("current = %v, want 30", m.CurrentMs)
	}
}

func TestRTTCollectorJitter(t *testing.T) {
	c := types.NewRTTCollector(100)
	c.AddSample(10.0)
	c.AddSample(20.0)
	c.AddSample(10.0)

	m := c.GetMetrics()
	// Jitter = mean consecutive diff = (|20-10| + |10-20|) / 2 = 10
	if m.JitterMs != 10.0 {
		t.Errorf("jitter = %v, want 10", m.JitterMs)
	}
}

func TestRTTCollectorSingleSample(t *testing.T) {
	c := types.NewRTTCollector(100)
	c.AddSample(5.5)

	m := c.GetMetrics()
	if m.JitterMs != 0 {
		t.Errorf("jitter with 1 sample = %v, want 0", m.JitterMs)
	}
	if m.Samples != 1 {
		t.Errorf("samples = %d, want 1", m.Samples)
	}
}

func TestRTTCollectorBaseline(t *testing.T) {
	c := types.NewRTTCollector(100)
	c.SetBaseline(5.0)

	m := c.GetMetrics()
	if m.BaselineMs != 5.0 {
		t.Errorf("baseline = %v, want 5", m.BaselineMs)
	}
}

func TestRTTCollectorRingBuffer(t *testing.T) {
	c := types.NewRTTCollector(5) // Ring buffer of 5

	// Add 8 samples — should keep last 5
	for i := 1; i <= 8; i++ {
		c.AddSample(float64(i))
	}

	m := c.GetMetrics()
	if m.Samples != 5 {
		t.Errorf("samples = %d, want 5", m.Samples)
	}
	if m.MinMs != 4.0 {
		t.Errorf("min = %v, want 4 (oldest kept)", m.MinMs)
	}
	if m.MaxMs != 8.0 {
		t.Errorf("max = %v, want 8", m.MaxMs)
	}
	if m.CurrentMs != 8.0 {
		t.Errorf("current = %v, want 8", m.CurrentMs)
	}
}

func TestRTTCollectorRingBufferAvg(t *testing.T) {
	c := types.NewRTTCollector(3)
	c.AddSample(100.0)
	c.AddSample(200.0)
	c.AddSample(300.0)
	c.AddSample(400.0) // evicts 100

	m := c.GetMetrics()
	// Should have 200, 300, 400
	expected := (200.0 + 300.0 + 400.0) / 3.0
	if math.Abs(m.AvgMs-expected) > 0.001 {
		t.Errorf("avg = %v, want %v", m.AvgMs, expected)
	}
}

func TestRTTCollectorDefaultMaxSamples(t *testing.T) {
	c := types.NewRTTCollector(0) // should default to 100
	for i := 0; i < 150; i++ {
		c.AddSample(float64(i))
	}

	m := c.GetMetrics()
	if m.Samples != 100 {
		t.Errorf("default max: samples = %d, want 100", m.Samples)
	}
}

func TestRTTCollectorConcurrent(t *testing.T) {
	c := types.NewRTTCollector(100)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			c.AddSample(v)
			c.GetMetrics()
		}(float64(i))
	}
	wg.Wait()

	m := c.GetMetrics()
	if m.Samples != 50 {
		t.Errorf("concurrent: samples = %d, want 50", m.Samples)
	}
}
