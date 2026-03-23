package api

import (
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

// BenchmarkToStreamSnapshotResponse maps live stream state to the JSON API DTO (polling + WS-adjacent paths).
func BenchmarkToStreamSnapshotResponse(b *testing.B) {
	snapshot := types.StreamSnapshot{
		Config: types.StreamConfig{
			ID:         "cfg-bench-1",
			Protocol:   types.ProtocolTCP,
			Direction:  types.DirectionDownload,
			Duration:   60 * time.Second,
			Streams:    4,
			PacketSize: 1400,
			ClientIP:   "192.0.2.10",
		},
		Status:    types.StreamStatusRunning,
		Progress:  42.5,
		Metrics:   benchSampleMetrics(),
		StartTime: time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = toStreamSnapshotResponse(snapshot)
	}
}
