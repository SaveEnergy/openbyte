package client

import (
	"fmt"
	"os"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (f *PlainFormatter) FormatProgress(progress, elapsed, remaining float64) {
	_ = progress
	_ = elapsed
	_ = remaining
}

func (f *PlainFormatter) FormatMetrics(metrics *types.Metrics) {
	_ = metrics
}

func (f *PlainFormatter) FormatComplete(results *StreamResults) {
	if results.Config != nil {
		fmt.Fprintf(f.writer, "stream_id=%s\n", results.StreamID)
		fmt.Fprintf(f.writer, "status=%s\n", results.Status)
		fmt.Fprintf(f.writer, "protocol=%s\n", results.Config.Protocol)
		fmt.Fprintf(f.writer, "direction=%s\n", results.Config.Direction)
		fmt.Fprintf(f.writer, "duration=%d\n", results.Config.Duration)
		fmt.Fprintf(f.writer, "streams=%d\n", results.Config.Streams)
	}
	if results.Results != nil {
		fmt.Fprintf(f.writer, "throughput_mbps=%.1f\n", results.Results.ThroughputMbps)
		fmt.Fprintf(f.writer, "throughput_avg_mbps=%.1f\n", results.Results.ThroughputAvgMbps)
		fmt.Fprintf(f.writer, "latency_min_ms=%.3f\n", results.Results.LatencyMs.MinMs)
		fmt.Fprintf(f.writer, "latency_max_ms=%.3f\n", results.Results.LatencyMs.MaxMs)
		fmt.Fprintf(f.writer, "latency_avg_ms=%.3f\n", results.Results.LatencyMs.AvgMs)
		fmt.Fprintf(f.writer, "latency_p50_ms=%.3f\n", results.Results.LatencyMs.P50Ms)
		fmt.Fprintf(f.writer, "latency_p95_ms=%.3f\n", results.Results.LatencyMs.P95Ms)
		fmt.Fprintf(f.writer, "latency_p99_ms=%.3f\n", results.Results.LatencyMs.P99Ms)
		fmt.Fprintf(f.writer, "jitter_ms=%.3f\n", results.Results.JitterMs)
		fmt.Fprintf(f.writer, "packet_loss_percent=%.2f\n", results.Results.PacketLossPercent)
		fmt.Fprintf(f.writer, "bytes_transferred=%d\n", results.Results.BytesTransferred)
		fmt.Fprintf(f.writer, "packets_sent=%d\n", results.Results.PacketsSent)
		fmt.Fprintf(f.writer, "packets_received=%d\n", results.Results.PacketsReceived)
	}
	if results.StartTime != "" {
		fmt.Fprintf(f.writer, "start_time=%s\n", results.StartTime)
	}
	if results.EndTime != "" {
		fmt.Fprintf(f.writer, "end_time=%s\n", results.EndTime)
	}
	if results.DurationSeconds > 0 {
		fmt.Fprintf(f.writer, "duration_seconds=%.1f\n", results.DurationSeconds)
	}
}

func (f *PlainFormatter) FormatError(err error) {
	fmt.Fprintf(os.Stderr, "openbyte client: error: %v\n", err)
}
