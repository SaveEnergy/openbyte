package client

import (
	"fmt"
	"os"
	"strings"

	"github.com/saveenergy/openbyte/pkg/types"
)

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
