package stream

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

// BenchmarkManagerGetStream is the hot read path for stream status polling.
func BenchmarkManagerGetStream(b *testing.B) {
	benchMuteStreamInfo(b)
	m := NewManager(256, 64)
	cfg := types.StreamConfig{
		Protocol:   types.ProtocolTCP,
		Direction:  types.DirectionDownload,
		Duration:   60 * time.Second,
		Streams:    1,
		PacketSize: 1400,
		ClientIP:   "192.0.2.1",
	}
	state, err := m.CreateStream(cfg)
	if err != nil {
		b.Fatal(err)
	}
	id := state.Config.ID

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := m.GetStream(id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkManagerSendMetricsUpdates scans active streams and enqueues metrics updates (broadcast tick body).
func BenchmarkManagerSendMetricsUpdates(b *testing.B) {
	benchMuteStreamInfo(b)
	m := NewManager(256, 128)
	const nStreams = 32
	ids := make([]string, 0, nStreams)
	for range nStreams {
		st, err := m.CreateStream(types.StreamConfig{
			Protocol:   types.ProtocolTCP,
			Direction:  types.DirectionDownload,
			Duration:   60 * time.Second,
			Streams:    2,
			PacketSize: 1400,
			ClientIP:   "192.0.2.1",
		})
		if err != nil {
			b.Fatal(err)
		}
		ids = append(ids, st.Config.ID)
	}
	for _, id := range ids {
		if err := m.StartStream(id); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	// Timer covers sendMetricsUpdates only; draining the buffered updates runs with the timer stopped.
	for range b.N {
		m.sendMetricsUpdates()
		b.StopTimer()
		for range nStreams {
			select {
			case <-m.metricsUpdateCh:
			default:
				b.Fatal("metrics channel underrun")
			}
		}
		b.StartTimer()
	}
}
