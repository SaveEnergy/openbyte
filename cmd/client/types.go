package client

import (
	"io"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
	"github.com/saveenergy/openbyte/pkg/types"
)

type OutputFormatter interface {
	FormatProgress(progress float64, elapsed, remaining float64)
	FormatMetrics(metrics *types.Metrics)
	FormatComplete(results *StreamResults)
	FormatError(err error)
}

type JSONFormatter struct {
	Writer io.Writer
}

type PlainFormatter struct {
	writer     io.Writer
	verbose    bool
	noColor    bool
	noProgress bool
}

func NewPlainFormatter(w io.Writer, verbose, noColor, noProgress bool) *PlainFormatter {
	return &PlainFormatter{writer: w, verbose: verbose, noColor: noColor, noProgress: noProgress}
}

type InteractiveFormatter struct {
	writer     io.Writer
	verbose    bool
	noColor    bool
	noProgress bool
}

func NewInteractiveFormatter(w io.Writer, verbose, noColor, noProgress bool) *InteractiveFormatter {
	return &InteractiveFormatter{writer: w, verbose: verbose, noColor: noColor, noProgress: noProgress}
}

// NDJSONFormatter emits newline-delimited JSON: one line per progress tick,
// one final line with the complete result. Machine-readable streaming output.
type NDJSONFormatter struct {
	Writer io.Writer
	errMu  sync.Mutex
	err    error
}

type Config struct {
	Direction  string
	Duration   int
	Streams    int
	ChunkSize  int
	ServerURL  string
	Timeout    int
	JSON       bool
	NDJSON     bool
	Plain      bool
	Verbose    bool
	Quiet      bool
	NoColor    bool
	NoProgress bool
	WarmUp     int
}

// SchemaVersion is the semantic version of the JSON output schema.
// Bump major on breaking changes; minor on additive changes.
const SchemaVersion = "2.0"

type StreamResults struct {
	SchemaVersion   string                     `json:"schema_version"`
	StreamID        string                     `json:"stream_id"`
	Status          string                     `json:"status"`
	Config          *StreamConfig              `json:"config,omitempty"`
	Results         *ResultMetrics             `json:"results,omitempty"`
	Interpretation  *diagnostic.Interpretation `json:"interpretation,omitempty"`
	StartTime       string                     `json:"start_time,omitempty"`
	EndTime         string                     `json:"end_time,omitempty"`
	DurationSeconds float64                    `json:"duration_seconds,omitempty"`
}

type StreamConfig struct {
	Protocol  string `json:"protocol"`
	Direction string `json:"direction"`
	Duration  int    `json:"duration"`
	Streams   int    `json:"streams"`
	ChunkSize int    `json:"chunk_size"`
	Server    string `json:"server,omitempty"`
}

// JSONErrorResponse is the structured error emitted when --json is active.
type JSONErrorResponse struct {
	SchemaVersion string `json:"schema_version"`
	Error         bool   `json:"error"`
	Code          string `json:"code"`
	Message       string `json:"message"`
}

type ResultMetrics struct {
	ThroughputMbps    float64              `json:"throughput_mbps"`
	ThroughputAvgMbps float64              `json:"throughput_avg_mbps"`
	LatencyMs         types.LatencyMetrics `json:"latency_ms"`
	RTT               types.RTTMetrics     `json:"rtt"`
	JitterMs          float64              `json:"jitter_ms"`
	PacketLossPercent float64              `json:"packet_loss_percent"`
	BytesTransferred  int64                `json:"bytes_transferred"`
	PacketsSent       int64                `json:"packets_sent"`
	PacketsReceived   int64                `json:"packets_received"`
	Network           *types.NetworkInfo   `json:"network,omitempty"`
}

type EngineMetrics struct {
	ThroughputMbps   float64
	BytesTransferred int64
	BytesSent        int64
	BytesReceived    int64
	Latency          LatencyStats
	RTT              types.RTTMetrics
	Network          *types.NetworkInfo
	JitterMs         float64
	Elapsed          time.Duration
	Running          bool
}

type LatencyStats struct {
	MinMs float64
	MaxMs float64
	AvgMs float64
	P50Ms float64
	P95Ms float64
	P99Ms float64
	Count int
}
