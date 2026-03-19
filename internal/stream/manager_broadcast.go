package stream

import (
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (m *Manager) broadcastMetrics() {
	defer m.wg.Done()
	interval := m.metricsUpdateInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.sendMetricsUpdates()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) sendMetricsUpdates() {
	m.mu.RLock()
	updates := make([]*MetricsUpdate, 0, len(m.streams))
	for streamID, state := range m.streams {
		snapshot := state.GetState()
		if snapshot.Status == types.StreamStatusRunning ||
			snapshot.Status == types.StreamStatusStarting ||
			snapshot.Status == types.StreamStatusCompleted ||
			snapshot.Status == types.StreamStatusFailed {
			updates = append(updates, &MetricsUpdate{
				StreamID: streamID,
				State:    snapshot,
			})
		}
	}
	m.mu.RUnlock()

	for _, update := range updates {
		select {
		case m.metricsUpdateCh <- update:
		default:
			atomic.AddInt64(&m.droppedMetricsUpdates, 1)
		}
	}
}
