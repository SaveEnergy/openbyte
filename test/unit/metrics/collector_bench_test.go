package metrics_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func BenchmarkCollectorGetMetrics(b *testing.B) {
	collector := metrics.NewCollector()
	for i := range 10000 {
		collector.RecordLatency(time.Duration(i%2000) * time.Millisecond)
		collector.RecordBytes(1024, "recv")
	}

	b.ResetTimer()
	for range b.N {
		_ = collector.GetMetrics()
	}
}
