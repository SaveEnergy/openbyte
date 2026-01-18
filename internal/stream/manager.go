package stream

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

type Manager struct {
	streams         map[string]*types.StreamState
	activeStreams   map[string]string
	activeByIP      map[string]int
	metricsUpdateCh chan *MetricsUpdate
	stopCh          chan struct{}
	wg              sync.WaitGroup
	mu              sync.RWMutex
	maxStreams      int
	maxStreamsPerIP int
	retentionPeriod time.Duration
	metricsUpdateInterval time.Duration
}

type MetricsUpdate struct {
	StreamID string
	State    types.StreamState
}

func NewManager(maxStreams int, maxStreamsPerIP int) *Manager {
	return &Manager{
		streams:         make(map[string]*types.StreamState),
		activeStreams:   make(map[string]string),
		activeByIP:      make(map[string]int),
		metricsUpdateCh: make(chan *MetricsUpdate, 100),
		stopCh:          make(chan struct{}),
		maxStreams:      maxStreams,
		maxStreamsPerIP: maxStreamsPerIP,
		retentionPeriod: 1 * time.Hour,
		metricsUpdateInterval: 1 * time.Second,
	}
}

func (m *Manager) SetRetentionPeriod(period time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retentionPeriod = period
}

func (m *Manager) SetMetricsUpdateInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsUpdateInterval = interval
}

func (m *Manager) GetMetricsUpdateChannel() <-chan *MetricsUpdate {
	return m.metricsUpdateCh
}

func (m *Manager) Start() {
	m.wg.Add(2)
	go m.cleanupExpiredStreams()
	go m.broadcastMetrics()
}

func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
	close(m.metricsUpdateCh)
}

func (m *Manager) CreateStream(config types.StreamConfig) (*types.StreamState, error) {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.StartTime.IsZero() {
		config.StartTime = time.Now()
	}

	m.mu.Lock()
	if len(m.streams) >= m.maxStreams {
		m.mu.Unlock()
		logging.Warn("Max concurrent streams reached",
			logging.Field{Key: "max", Value: m.maxStreams},
			logging.Field{Key: "active", Value: len(m.streams)})
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
	activeCount := len(m.streams)
	m.mu.Unlock()

	logging.Info("Stream created",
		logging.Field{Key: "id", Value: config.ID},
		logging.Field{Key: "direction", Value: string(config.Direction)},
		logging.Field{Key: "duration", Value: config.Duration.Seconds()},
		logging.Field{Key: "streams", Value: config.Streams},
		logging.Field{Key: "active", Value: activeCount})

	return state, nil
}

func (m *Manager) StartStream(streamID string) error {
	m.mu.RLock()
	state, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		return errors.ErrStreamNotFound(streamID)
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
	m.mu.RLock()
	state, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	state.UpdateStatus(types.StreamStatusCancelled)
	m.releaseActiveStream(streamID)
	return nil
}

func (m *Manager) CompleteStream(streamID string, metrics types.Metrics) error {
	m.mu.RLock()
	state, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	state.UpdateMetrics(metrics)
	state.UpdateStatus(types.StreamStatusCompleted)
	m.releaseActiveStream(streamID)

	logging.Info("Stream completed",
		logging.Field{Key: "id", Value: streamID},
		logging.Field{Key: "throughput_mbps", Value: metrics.ThroughputMbps})

	return nil
}

func (m *Manager) FailStream(streamID string, metrics types.Metrics) error {
	m.mu.RLock()
	state, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	state.UpdateMetrics(metrics)
	state.UpdateStatus(types.StreamStatusFailed)
	m.releaseActiveStream(streamID)

	logging.Info("Stream failed",
		logging.Field{Key: "id", Value: streamID})

	return nil
}

func (m *Manager) UpdateMetrics(streamID string, metrics types.Metrics) error {
	m.mu.RLock()
	state, exists := m.streams[streamID]
	m.mu.RUnlock()

	if !exists {
		return errors.ErrStreamNotFound(streamID)
	}

	state.UpdateMetrics(metrics)
	return nil
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
	return len(m.GetActiveStreams())
}

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
	retentionPeriod := m.retentionPeriod

	now := time.Now()

	for streamID, state := range m.streams {
		snapshot := state.GetState()

		if snapshot.Status == types.StreamStatusRunning || snapshot.Status == types.StreamStatusStarting {
			maxDuration := snapshot.Config.Duration + 30*time.Second
			if now.Sub(snapshot.StartTime) > maxDuration {
				state.UpdateStatus(types.StreamStatusFailed)
				m.releaseActiveStreamLocked(streamID)
				delete(m.streams, streamID)
				continue
			}
		}

		if snapshot.Status == types.StreamStatusCompleted ||
			snapshot.Status == types.StreamStatusFailed ||
			snapshot.Status == types.StreamStatusCancelled {
			if !snapshot.EndTime.IsZero() && now.Sub(snapshot.EndTime) > retentionPeriod {
				m.releaseActiveStreamLocked(streamID)
				delete(m.streams, streamID)
			}
		}
	}
	m.mu.Unlock()
}

func (m *Manager) releaseActiveStream(streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.releaseActiveStreamLocked(streamID)
}

func (m *Manager) releaseActiveStreamLocked(streamID string) {
	clientIP, exists := m.activeStreams[streamID]
	if !exists {
		return
	}
	delete(m.activeStreams, streamID)

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
	defer m.mu.RUnlock()

	for streamID, state := range m.streams {
		snapshot := state.GetState()
		if snapshot.Status == types.StreamStatusRunning ||
			snapshot.Status == types.StreamStatusStarting ||
			snapshot.Status == types.StreamStatusCompleted ||
			snapshot.Status == types.StreamStatusFailed {
			select {
			case m.metricsUpdateCh <- &MetricsUpdate{
				StreamID: streamID,
				State:    snapshot,
			}:
			default:
			}
		}
	}
}
