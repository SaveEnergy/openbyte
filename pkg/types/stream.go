package types

import (
	"sync"
	"time"
)

type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolQUIC Protocol = "quic"
)

type Direction string

const (
	DirectionDownload      Direction = "download"
	DirectionUpload        Direction = "upload"
	DirectionBidirectional Direction = "bidirectional"
)

type StreamStatus string

const (
	StreamStatusPending   StreamStatus = "pending"
	StreamStatusStarting  StreamStatus = "starting"
	StreamStatusRunning   StreamStatus = "running"
	StreamStatusCompleted StreamStatus = "completed"
	StreamStatusFailed    StreamStatus = "failed"
	StreamStatusCancelled StreamStatus = "cancelled"
)

type StreamConfig struct {
	ID         string        `json:"id"`
	Protocol   Protocol      `json:"protocol"`
	Direction  Direction     `json:"direction"`
	Duration   time.Duration `json:"duration"`
	Streams    int           `json:"streams"`
	PacketSize int           `json:"packet_size"`
	StartTime  time.Time     `json:"start_time,omitempty"`
	ClientIP   string        `json:"client_ip,omitempty"`
}

type StreamState struct {
	Config      StreamConfig
	Status      StreamStatus
	Progress    float64
	Metrics     Metrics
	Network     *NetworkInfo
	Connections []ConnectionState
	StartTime   time.Time
	EndTime     time.Time
	Error       error
	mu          sync.RWMutex
}

func (ss *StreamState) UpdateStatus(status StreamStatus) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Status = status
	if status == StreamStatusRunning && ss.StartTime.IsZero() {
		ss.StartTime = time.Now()
	}
	if status == StreamStatusCompleted || status == StreamStatusFailed || status == StreamStatusCancelled {
		ss.EndTime = time.Now()
	}
}

func (ss *StreamState) UpdateMetrics(m Metrics) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Metrics = m
	elapsed := time.Since(ss.StartTime)
	if !ss.StartTime.IsZero() && ss.Config.Duration > 0 {
		ss.Progress = float64(elapsed) / float64(ss.Config.Duration) * 100
		if ss.Progress > 100 {
			ss.Progress = 100
		}
	}
}

func (ss *StreamState) UpdateConnections(connections []ConnectionState) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Connections = connections
}

func (ss *StreamState) GetState() StreamState {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return StreamState{
		Config:      ss.Config,
		Status:      ss.Status,
		Progress:    ss.Progress,
		Metrics:     ss.Metrics,
		Connections: ss.Connections,
		StartTime:   ss.StartTime,
		EndTime:     ss.EndTime,
		Error:       ss.Error,
	}
}
