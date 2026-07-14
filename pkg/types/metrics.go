package types

import "time"

type Metrics struct {
	ThroughputMbps    float64        `json:"throughput_mbps"`
	ThroughputAvgMbps float64        `json:"throughput_avg_mbps"`
	Latency           LatencyMetrics `json:"latency_ms"`
	JitterMs          float64        `json:"jitter_ms"`
	BytesTransferred  int64          `json:"bytes_transferred"`
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
