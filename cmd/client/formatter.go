package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/saveenergy/openbyte/pkg/types"
)

// classifyErrorCode maps an error to a machine-readable error code for JSON output.
func classifyErrorCode(err error) string {
	if err == nil {
		return "unknown"
	}

	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Op == "dial" {
			return "connection_refused"
		}
		if netErr.Timeout() {
			return "timeout"
		}
		return "network_error"
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "connection refused"):
		return "connection_refused"
	case strings.Contains(msg, "no such host"):
		return "server_unavailable"
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit"):
		return "rate_limited"
	case strings.Contains(msg, "503") || strings.Contains(msg, "server at capacity"):
		return "server_unavailable"
	case strings.Contains(msg, "invalid") || strings.Contains(msg, "must be"):
		return "invalid_config"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	default:
		return "unknown"
	}
}

func (f *JSONFormatter) FormatProgress(progress, elapsed, remaining float64) {}

func (f *JSONFormatter) FormatMetrics(metrics *types.Metrics) {}

func (f *JSONFormatter) FormatComplete(results *StreamResults) {
	if err := json.NewEncoder(f.Writer).Encode(results); err != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: error encoding JSON: %v\n", err)
	}
}

func (f *JSONFormatter) FormatError(err error) {
	errResp := JSONErrorResponse{
		SchemaVersion: SchemaVersion,
		Error:         true,
		Code:          classifyErrorCode(err),
		Message:       err.Error(),
	}
	if encErr := json.NewEncoder(f.Writer).Encode(errResp); encErr != nil {
		fmt.Fprintf(os.Stderr, "openbyte client: error encoding JSON: %v\n", encErr)
	}
}

func (f *PlainFormatter) FormatProgress(progress, elapsed, remaining float64) {}

func (f *PlainFormatter) FormatMetrics(metrics *types.Metrics) {}

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

func (f *InteractiveFormatter) FormatProgress(progress, elapsed, remaining float64) {
	if f.noProgress {
		return
	}
	barWidth := 30
	filled := int(progress / 100 * float64(barWidth))

	var bar string
	if !f.noColor {
		bar = strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		if progress < 50 {
			bar = fmt.Sprintf("\033[33m%s\033[0m", bar)
		} else if progress < 90 {
			bar = fmt.Sprintf("\033[36m%s\033[0m", bar)
		} else {
			bar = fmt.Sprintf("\033[32m%s\033[0m", bar)
		}
	} else {
		bar = strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	}

	fmt.Fprintf(f.writer, "\rProgress: [%s] %.1f%% (%.1fs elapsed, %.1fs remaining)", bar, progress, elapsed, remaining)
	if progress >= 100 {
		fmt.Fprintln(f.writer)
	}
}

func (f *InteractiveFormatter) FormatMetrics(metrics *types.Metrics) {
	if !f.verbose {
		return
	}
	fmt.Fprintf(f.writer, "  Throughput: %.1f Mbps\n", metrics.ThroughputMbps)
}

func (f *InteractiveFormatter) FormatComplete(results *StreamResults) {
	fmt.Fprintln(f.writer, "\nResults:")
	if results.Results == nil {
		return
	}
	r := results.Results
	c := func(code, label string) string {
		if f.noColor {
			return label
		}
		return fmt.Sprintf("\033[%sm%s\033[0m", code, label)
	}

	fmt.Fprintf(f.writer, " %s %.1f Mbps (peak)\n", c("36", "Throughput:"), r.ThroughputMbps)
	fmt.Fprintf(f.writer, "  %.1f Mbps (average)\n", r.ThroughputAvgMbps)
	fmt.Fprintf(f.writer, " %s %.3f ms (avg)\n", c("33", "Latency:"), r.LatencyMs.AvgMs)
	fmt.Fprintf(f.writer, "  %.3f ms (min)\n", r.LatencyMs.MinMs)
	fmt.Fprintf(f.writer, "  %.3f ms (max)\n", r.LatencyMs.MaxMs)
	fmt.Fprintf(f.writer, "  %.3f ms (p50)\n", r.LatencyMs.P50Ms)
	fmt.Fprintf(f.writer, "  %.3f ms (p95)\n", r.LatencyMs.P95Ms)
	fmt.Fprintf(f.writer, "  %.3f ms (p99)\n", r.LatencyMs.P99Ms)
	fmt.Fprintf(f.writer, " %s %.3f ms\n", c("35", "Jitter:"), r.JitterMs)
	lossColor := "32"
	if r.PacketLossPercent > 1.0 {
		lossColor = "31"
	}
	fmt.Fprintf(f.writer, " %s %.2f%%\n", c(lossColor, "Packet Loss:"), r.PacketLossPercent)
	fmt.Fprintf(f.writer, " %s %s transferred\n", c("37", "Bytes:"), formatBytes(r.BytesTransferred))
	fmt.Fprintf(f.writer, " %s %s sent\n", c("37", "Packets:"), formatNumber(r.PacketsSent))
	fmt.Fprintf(f.writer, "  %s received\n", formatNumber(r.PacketsReceived))
}

func (f *InteractiveFormatter) FormatError(err error) {
	fmt.Fprintf(os.Stderr, "openbyte client: error: %v\n", err)
}

// NDJSON formatter — newline-delimited JSON for streaming progress.

func (f *NDJSONFormatter) FormatProgress(progress, elapsed, remaining float64) {
	msg := map[string]interface{}{
		"type":        "progress",
		"percent":     progress,
		"elapsed_s":   elapsed,
		"remaining_s": remaining,
	}
	if err := json.NewEncoder(f.Writer).Encode(msg); err != nil {
		fmt.Fprintf(os.Stderr, "ndjson encode error: %v\n", err)
	}
}

func (f *NDJSONFormatter) FormatMetrics(metrics *types.Metrics) {
	msg := map[string]interface{}{
		"type":            "metrics",
		"throughput_mbps": metrics.ThroughputMbps,
		"bytes":           metrics.BytesTransferred,
		"latency_avg_ms":  metrics.Latency.AvgMs,
		"jitter_ms":       metrics.JitterMs,
	}
	if err := json.NewEncoder(f.Writer).Encode(msg); err != nil {
		fmt.Fprintf(os.Stderr, "ndjson encode error: %v\n", err)
	}
}

func (f *NDJSONFormatter) FormatComplete(results *StreamResults) {
	msg := map[string]interface{}{
		"type": "result",
		"data": results,
	}
	if err := json.NewEncoder(f.Writer).Encode(msg); err != nil {
		fmt.Fprintf(os.Stderr, "ndjson encode error: %v\n", err)
	}
}

func (f *NDJSONFormatter) FormatError(err error) {
	msg := JSONErrorResponse{
		SchemaVersion: SchemaVersion,
		Error:         true,
		Code:          classifyErrorCode(err),
		Message:       err.Error(),
	}
	if encErr := json.NewEncoder(f.Writer).Encode(msg); encErr != nil {
		fmt.Fprintf(os.Stderr, "ndjson encode error: %v\n", encErr)
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	start := 0
	var result strings.Builder
	if len(s) > 0 && s[0] == '-' {
		result.WriteByte('-')
		start = 1
	}
	digits := s[start:]
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(r)
	}
	return result.String()
}
