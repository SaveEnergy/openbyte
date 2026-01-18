package metrics

import (
	"testing"
	"time"
)

func BenchmarkCollectorGetMetrics(b *testing.B) {
	collector := NewCollector()
	for i := 0; i < 10000; i++ {
		collector.RecordLatency(time.Duration(i%2000) * time.Millisecond)
		collector.RecordBytes(1024, "recv")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = collector.GetMetrics()
	}
}
