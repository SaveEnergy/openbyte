// BenchmarkManagerCompleteStream and BenchmarkManagerFailStreamWithError hit production
// logging on Complete/Fail; mute Info during these benches so perf-record does not write
// unbounded log output to build/perf/bench.txt (go test captures stderr with stdout).
package stream

import (
	"errors"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

func benchMuteStreamInfo(b *testing.B) {
	logging.GetLogger().SetLevel(logging.LevelError)
	b.Cleanup(func() { logging.GetLogger().SetLevel(logging.LevelInfo) })
}

func benchStreamConfig() types.StreamConfig {
	return types.StreamConfig{
		Protocol:   types.ProtocolTCP,
		Direction:  types.DirectionDownload,
		Duration:   60 * time.Second,
		Streams:    2,
		PacketSize: 1400,
		ClientIP:   "192.0.2.1",
	}
}

func benchMetricsTick() types.Metrics {
	return types.Metrics{
		ThroughputMbps:    100.5,
		ThroughputAvgMbps: 99.0,
		Latency: types.LatencyMetrics{
			MinMs: 1, MaxMs: 4, AvgMs: 2, P50Ms: 2, P95Ms: 3, P99Ms: 4,
			Count: 50,
		},
		JitterMs:          0.1,
		PacketLossPercent: 0,
		BytesTransferred:  1024,
		PacketsSent:       10,
		PacketsReceived:   10,
		Timestamp:         time.Now(),
		StreamCount:       2,
	}
}

// BenchmarkManagerUpdateMetrics is POST /stream/{id}/metrics (manager lock + StreamState.UpdateMetrics).
func BenchmarkManagerUpdateMetrics(b *testing.B) {
	benchMuteStreamInfo(b)
	m := NewManager(512, 128)
	st, err := m.CreateStream(benchStreamConfig())
	if err != nil {
		b.Fatal(err)
	}
	id := st.Config.ID
	if err := m.StartStream(id); err != nil {
		b.Fatal(err)
	}
	metrics := benchMetricsTick()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := m.UpdateMetrics(id, metrics); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkManagerUpdateMetricsParallel measures lock contention when many goroutines update the same stream.
func BenchmarkManagerUpdateMetricsParallel(b *testing.B) {
	benchMuteStreamInfo(b)
	m := NewManager(512, 128)
	st, err := m.CreateStream(benchStreamConfig())
	if err != nil {
		b.Fatal(err)
	}
	id := st.Config.ID
	if err := m.StartStream(id); err != nil {
		b.Fatal(err)
	}
	metrics := benchMetricsTick()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := m.UpdateMetrics(id, metrics); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkManagerCancelStream measures new stream + StartStream + CancelStream per iteration.
// Do not use StopTimer around NewManager/CreateStream/StartStream: the runtime scales b.N from the
// timed slice only, so cheap CancelStream drives N into millions while untimed setup runs N times
// and the bench appears hung.
func BenchmarkManagerCancelStream(b *testing.B) {
	benchMuteStreamInfo(b)
	cfg := benchStreamConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m := NewManager(1024, 256)
		st, err := m.CreateStream(cfg)
		if err != nil {
			b.Fatal(err)
		}
		id := st.Config.ID
		if err := m.StartStream(id); err != nil {
			b.Fatal(err)
		}
		if err := m.CancelStream(id); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkManagerCompleteStream measures full stream lifecycle through CompleteStream (see CancelStream re StopTimer).
func BenchmarkManagerCompleteStream(b *testing.B) {
	benchMuteStreamInfo(b)
	cfg := benchStreamConfig()
	metrics := benchMetricsTick()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m := NewManager(1024, 256)
		st, err := m.CreateStream(cfg)
		if err != nil {
			b.Fatal(err)
		}
		id := st.Config.ID
		if err := m.StartStream(id); err != nil {
			b.Fatal(err)
		}
		if err := m.CompleteStream(id, metrics); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkManagerFailStreamWithError measures full lifecycle through FailStreamWithError (see CancelStream re StopTimer).
func BenchmarkManagerFailStreamWithError(b *testing.B) {
	benchMuteStreamInfo(b)
	cfg := benchStreamConfig()
	metrics := benchMetricsTick()
	cause := errors.New("bench failure")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		m := NewManager(1024, 256)
		st, err := m.CreateStream(cfg)
		if err != nil {
			b.Fatal(err)
		}
		id := st.Config.ID
		if err := m.StartStream(id); err != nil {
			b.Fatal(err)
		}
		if err := m.FailStreamWithError(id, metrics, cause); err != nil {
			b.Fatal(err)
		}
	}
}
