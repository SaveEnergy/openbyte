package stream

import (
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

func (m *Manager) CreateStream(config types.StreamConfig) (*types.StreamState, error) {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.StartTime.IsZero() {
		config.StartTime = time.Now()
	}

	m.mu.Lock()
	currentActive := int(atomic.LoadInt64(&m.activeCount))
	if currentActive >= m.maxStreams {
		m.mu.Unlock()
		logging.Warn("Max concurrent streams reached",
			logging.Field{Key: "max", Value: m.maxStreams},
			logging.Field{Key: "active", Value: currentActive})
		return nil, errors.ErrResourceExhausted("max concurrent streams reached")
	}
	if _, exists := m.streams[config.ID]; exists {
		m.mu.Unlock()
		return nil, errors.ErrStreamAlreadyExists(config.ID)
	}
	clientIP := config.ClientIP
	if clientIP == "" {
		clientIP = "unknown"
	}
	if m.maxStreamsPerIP > 0 {
		if m.activeByIP[clientIP] >= m.maxStreamsPerIP {
			m.mu.Unlock()
			return nil, errors.ErrResourceExhausted("max concurrent streams per IP reached")
		}
	}

	state := &types.StreamState{
		Config: config,
		Status: types.StreamStatusPending,
	}
	m.streams[config.ID] = state
	m.activeStreams[config.ID] = clientIP
	m.activeByIP[clientIP]++
	atomic.AddInt64(&m.activeCount, 1)
	m.mu.Unlock()

	logging.Info("Stream created",
		logging.Field{Key: "id", Value: config.ID},
		logging.Field{Key: "direction", Value: string(config.Direction)},
		logging.Field{Key: "duration", Value: config.Duration.Seconds()},
		logging.Field{Key: "streams", Value: config.Streams},
		logging.Field{Key: "active", Value: atomic.LoadInt64(&m.activeCount)})

	return state, nil
}

func (m *Manager) StartStream(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.streams[streamID]
	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	current := state.GetState().Status
	if isTerminalStatus(current) {
		return errors.ErrInvalidConfig("cannot start terminal stream", nil)
	}
	state.UpdateStatus(types.StreamStatusRunning)
	return nil
}

func (m *Manager) GetStream(streamID string) (*types.StreamState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, exists := m.streams[streamID]
	if !exists {
		return nil, errors.ErrStreamNotFound(streamID)
	}
	return state, nil
}

func (m *Manager) CancelStream(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.streams[streamID]
	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	current := state.GetState().Status
	if isTerminalStatus(current) {
		if current == types.StreamStatusCancelled {
			return nil
		}
		return errors.ErrInvalidConfig(errStreamAlreadyFinalized, nil)
	}
	state.UpdateStatus(types.StreamStatusCancelled)
	m.releaseActiveStreamLocked(streamID)
	return nil
}

func (m *Manager) CompleteStream(streamID string, metrics types.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.streams[streamID]
	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	current := state.GetState().Status
	if isTerminalStatus(current) {
		if current == types.StreamStatusCompleted {
			return nil
		}
		return errors.ErrInvalidConfig(errStreamAlreadyFinalized, nil)
	}
	state.UpdateMetrics(metrics)
	state.UpdateStatus(types.StreamStatusCompleted)
	m.releaseActiveStreamLocked(streamID)

	logging.Info("Stream completed",
		logging.Field{Key: "id", Value: streamID},
		logging.Field{Key: "throughput_mbps", Value: metrics.ThroughputMbps})

	return nil
}

func (m *Manager) FailStream(streamID string, metrics types.Metrics) error {
	return m.FailStreamWithError(streamID, metrics, nil)
}

func (m *Manager) FailStreamWithError(streamID string, metrics types.Metrics, cause error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.streams[streamID]
	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	current := state.GetState().Status
	if isTerminalStatus(current) {
		if current == types.StreamStatusFailed {
			return nil
		}
		return errors.ErrInvalidConfig(errStreamAlreadyFinalized, nil)
	}
	state.UpdateMetrics(metrics)
	state.SetError(cause)
	state.UpdateStatus(types.StreamStatusFailed)
	m.releaseActiveStreamLocked(streamID)

	logging.Info("Stream failed",
		logging.Field{Key: "id", Value: streamID},
		logging.Field{Key: "throughput_mbps", Value: metrics.ThroughputMbps},
		logging.Field{Key: "bytes_transferred", Value: metrics.BytesTransferred},
		logging.Field{Key: "reason", Value: failureReason(cause)})

	return nil
}

func failureReason(cause error) string {
	if cause == nil {
		return defaultFailureReason
	}
	if message := cause.Error(); message != "" {
		return message
	}
	return defaultFailureReason
}

func (m *Manager) UpdateMetrics(streamID string, metrics types.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.streams[streamID]
	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}
	if isTerminalStatus(state.GetState().Status) {
		return errors.ErrInvalidConfig("cannot update metrics for terminal stream", nil)
	}

	state.UpdateMetrics(metrics)
	return nil
}

func isTerminalStatus(status types.StreamStatus) bool {
	return status == types.StreamStatusCompleted ||
		status == types.StreamStatusFailed ||
		status == types.StreamStatusCancelled
}

func (m *Manager) GetActiveStreams() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := make([]string, 0, len(m.streams))
	for id, state := range m.streams {
		snapshot := state.GetState()
		if snapshot.Status == types.StreamStatusRunning ||
			snapshot.Status == types.StreamStatusStarting ||
			snapshot.Status == types.StreamStatusPending {
			active = append(active, id)
		}
	}
	return active
}

func (m *Manager) ActiveCount() int {
	return int(atomic.LoadInt64(&m.activeCount))
}

func (m *Manager) DroppedMetricsUpdates() int64 {
	return atomic.LoadInt64(&m.droppedMetricsUpdates)
}
