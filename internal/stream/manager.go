package stream

import (
	"sync"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

const errStreamAlreadyFinalized = "stream already finalized"

const defaultFailureReason = "reported_failed_status"

type Manager struct {
	streams               map[string]*types.StreamState
	activeStreams         map[string]string
	activeByIP            map[string]int
	metricsUpdateCh       chan *MetricsUpdate
	stopCh                chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.RWMutex
	activeCount           int64
	maxStreams            int
	maxStreamsPerIP       int
	retentionPeriod       time.Duration
	metricsUpdateInterval time.Duration
	stopOnce              sync.Once
	droppedMetricsUpdates int64
}

type MetricsUpdate struct {
	StreamID string
	State    types.StreamSnapshot
}

func NewManager(maxStreams, maxStreamsPerIP int) *Manager {
	return &Manager{
		streams:               make(map[string]*types.StreamState),
		activeStreams:         make(map[string]string),
		activeByIP:            make(map[string]int),
		metricsUpdateCh:       make(chan *MetricsUpdate, 100),
		stopCh:                make(chan struct{}),
		maxStreams:            maxStreams,
		maxStreamsPerIP:       maxStreamsPerIP,
		retentionPeriod:       1 * time.Hour,
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
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
	close(m.metricsUpdateCh)
}
