package websocket

import (
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type wsMessage struct {
	Type             string        `json:"type"`
	StreamID         string        `json:"stream_id"`
	Status           string        `json:"status"`
	Progress         float64       `json:"progress,omitempty"`
	ElapsedSeconds   float64       `json:"elapsed_seconds,omitempty"`
	RemainingSeconds float64       `json:"remaining_seconds,omitempty"`
	Metrics          types.Metrics `json:"metrics"`
	Time             int64         `json:"time"`
	Results          *wsResults    `json:"results,omitempty"`
	Error            string        `json:"error,omitempty"`
	Message          string        `json:"message,omitempty"`
}

type wsResults struct {
	StreamID        string          `json:"stream_id"`
	Status          string          `json:"status"`
	Config          wsResultConfig  `json:"config"`
	Results         wsResultMetrics `json:"results"`
	StartTime       string          `json:"start_time"`
	EndTime         string          `json:"end_time"`
	DurationSeconds float64         `json:"duration_seconds"`
}

type wsResultConfig struct {
	Protocol   string `json:"protocol"`
	Direction  string `json:"direction"`
	Duration   int    `json:"duration"`
	Streams    int    `json:"streams"`
	PacketSize int    `json:"packet_size"`
}

type wsResultMetrics struct {
	ThroughputMbps    float64              `json:"throughput_mbps"`
	ThroughputAvgMbps float64              `json:"throughput_avg_mbps"`
	Latency           types.LatencyMetrics `json:"latency_ms"`
	JitterMs          float64              `json:"jitter_ms"`
	PacketLossPercent float64              `json:"packet_loss_percent"`
	BytesTransferred  int64                `json:"bytes_transferred"`
	PacketsSent       int64                `json:"packets_sent"`
	PacketsReceived   int64                `json:"packets_received"`
}

func (c *clientConn) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(v)
}

func (c *clientConn) writeMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteMessage(messageType, data)
}
