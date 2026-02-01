package client

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (f *JSONFormatter) FormatProgress(progress, elapsed, remaining float64) {}

func (f *JSONFormatter) FormatMetrics(metrics *types.Metrics) {}

func (f *JSONFormatter) FormatComplete(results *StreamResults) {
	json.NewEncoder(f.writer).Encode(results)
}

func (f *JSONFormatter) FormatError(err error) {
		fmt.Fprintf(os.Stderr, "openbyte client: error: %v\n", err)
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
	if results.Results != nil {
		if !f.noColor {
			fmt.Fprintf(f.writer, " \033[36mThroughput:\033[0m %.1f Mbps (peak)\n", results.Results.ThroughputMbps)
			fmt.Fprintf(f.writer, "  %.1f Mbps (average)\n", results.Results.ThroughputAvgMbps)
			fmt.Fprintf(f.writer, " \033[33mLatency:\033[0m %.3f ms (avg)\n", results.Results.LatencyMs.AvgMs)
			fmt.Fprintf(f.writer, "  %.3f ms (min)\n", results.Results.LatencyMs.MinMs)
			fmt.Fprintf(f.writer, "  %.3f ms (max)\n", results.Results.LatencyMs.MaxMs)
			fmt.Fprintf(f.writer, "  %.3f ms (p50)\n", results.Results.LatencyMs.P50Ms)
			fmt.Fprintf(f.writer, "  %.3f ms (p95)\n", results.Results.LatencyMs.P95Ms)
			fmt.Fprintf(f.writer, "  %.3f ms (p99)\n", results.Results.LatencyMs.P99Ms)
			fmt.Fprintf(f.writer, " \033[35mJitter:\033[0m %.3f ms\n", results.Results.JitterMs)
			if results.Results.PacketLossPercent > 1.0 {
				fmt.Fprintf(f.writer, " \033[31mPacket Loss:\033[0m %.2f%%\n", results.Results.PacketLossPercent)
			} else {
				fmt.Fprintf(f.writer, " \033[32mPacket Loss:\033[0m %.2f%%\n", results.Results.PacketLossPercent)
			}
			fmt.Fprintf(f.writer, " \033[37mBytes:\033[0m %s transferred\n", formatBytes(results.Results.BytesTransferred))
			fmt.Fprintf(f.writer, " \033[37mPackets:\033[0m %s sent\n", formatNumber(results.Results.PacketsSent))
			fmt.Fprintf(f.writer, "  %s received\n", formatNumber(results.Results.PacketsReceived))
		} else {
			fmt.Fprintf(f.writer, " Throughput: %.1f Mbps (peak)\n", results.Results.ThroughputMbps)
			fmt.Fprintf(f.writer, "  %.1f Mbps (average)\n", results.Results.ThroughputAvgMbps)
			fmt.Fprintf(f.writer, " Latency: %.3f ms (avg)\n", results.Results.LatencyMs.AvgMs)
			fmt.Fprintf(f.writer, "  %.3f ms (min)\n", results.Results.LatencyMs.MinMs)
			fmt.Fprintf(f.writer, "  %.3f ms (max)\n", results.Results.LatencyMs.MaxMs)
			fmt.Fprintf(f.writer, "  %.3f ms (p50)\n", results.Results.LatencyMs.P50Ms)
			fmt.Fprintf(f.writer, "  %.3f ms (p95)\n", results.Results.LatencyMs.P95Ms)
			fmt.Fprintf(f.writer, "  %.3f ms (p99)\n", results.Results.LatencyMs.P99Ms)
			fmt.Fprintf(f.writer, " Jitter: %.3f ms\n", results.Results.JitterMs)
			fmt.Fprintf(f.writer, " Packet Loss: %.2f%%\n", results.Results.PacketLossPercent)
			fmt.Fprintf(f.writer, " Bytes: %s transferred\n", formatBytes(results.Results.BytesTransferred))
			fmt.Fprintf(f.writer, " Packets: %s sent\n", formatNumber(results.Results.PacketsSent))
			fmt.Fprintf(f.writer, "  %s received\n", formatNumber(results.Results.PacketsReceived))
		}
	}
}

func (f *InteractiveFormatter) FormatError(err error) {
		fmt.Fprintf(os.Stderr, "openbyte client: error: %v\n", err)
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
	var result strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(r)
	}
	return result.String()
}
