package metrics_test

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/metrics"
)

func BenchmarkCollectorGetMetrics(b *testing.B) {
	collector := metrics.NewCollector()
	for i := 0; i < 10000; i++ {
		collector.RecordLatency(time.Duration(i%2000) * time.Millisecond)
		collector.RecordBytes(1024, "recv")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.GetMetrics()
	}
}
