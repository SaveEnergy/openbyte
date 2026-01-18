package types

import "time"

type Metrics struct {
	ThroughputMbps    float64        `json:"throughput_mbps"`
	ThroughputAvgMbps float64        `json:"throughput_avg_mbps"`
	Latency           LatencyMetrics `json:"latency_ms"`
	RTT               RTTMetrics     `json:"rtt"`
	JitterMs          float64        `json:"jitter_ms"`
	PacketLossPercent float64        `json:"packet_loss_percent"`
	BytesTransferred  int64          `json:"bytes_transferred"`
	PacketsSent       int64          `json:"packets_sent"`
	PacketsReceived   int64          `json:"packets_received"`
	Timestamp         time.Time      `json:"timestamp"`
	StreamCount       int            `json:"stream_count"`
}

type LatencyMetrics struct {
	MinMs float64 `json:"min_ms"`
	MaxMs float64 `json:"max_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
	Count int     `json:"count"`
}

type RTTMetrics struct {
	BaselineMs float64 `json:"baseline_ms"`
	CurrentMs  float64 `json:"current_ms"`
	MinMs      float64 `json:"min_ms"`
	MaxMs      float64 `json:"max_ms"`
	AvgMs      float64 `json:"avg_ms"`
	JitterMs   float64 `json:"jitter_ms"`
	Samples    int     `json:"samples"`
}
