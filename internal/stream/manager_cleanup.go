package stream

import (
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (m *Manager) cleanupExpiredStreams() {
	defer m.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	retentionPeriod := m.retentionPeriod
	now := time.Now()
	for streamID, state := range m.streams {
		snapshot := state.GetState()
		switch snapshot.Status {
		case types.StreamStatusRunning, types.StreamStatusStarting:
			maxDuration := snapshot.Config.Duration + 30*time.Second
			if now.Sub(snapshot.StartTime) > maxDuration {
				state.UpdateStatus(types.StreamStatusFailed)
				m.releaseActiveStreamLocked(streamID)
				delete(m.streams, streamID)
			}
		case types.StreamStatusPending:
			// Pending streams that were never started — clean up after 30s
			if now.Sub(snapshot.Config.StartTime) > 30*time.Second {
				state.UpdateStatus(types.StreamStatusFailed)
				m.releaseActiveStreamLocked(streamID)
				delete(m.streams, streamID)
			}
		case types.StreamStatusCompleted, types.StreamStatusFailed, types.StreamStatusCancelled:
			if !snapshot.EndTime.IsZero() && now.Sub(snapshot.EndTime) > retentionPeriod {
				m.releaseActiveStreamLocked(streamID)
				delete(m.streams, streamID)
			}
		}
	}
}

func (m *Manager) releaseActiveStreamLocked(streamID string) {
	clientIP, exists := m.activeStreams[streamID]
	if !exists {
		return
	}
	delete(m.activeStreams, streamID)
	atomic.AddInt64(&m.activeCount, -1)

	if clientIP == "" {
		clientIP = "unknown"
	}
	current := m.activeByIP[clientIP]
	if current <= 1 {
		delete(m.activeByIP, clientIP)
		return
	}
	m.activeByIP[clientIP] = current - 1
}
