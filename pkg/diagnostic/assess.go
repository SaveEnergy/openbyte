package diagnostic

import (
	"fmt"
	"strings"
)

func suitability(p Params) []string {
	s := []string{}

	if (p.DownloadMbps >= 1 || p.UploadMbps >= 1) && p.LatencyMs < 200 {
		s = append(s, "web_browsing")
	}

	if p.DownloadMbps >= 5 && p.UploadMbps >= 2 && p.LatencyMs < 100 && p.JitterMs < 30 {
		s = append(s, "video_conferencing")
	}

	if p.DownloadMbps >= 25 {
		s = append(s, "streaming_4k")
	} else if p.DownloadMbps >= 5 {
		s = append(s, "streaming_hd")
	}

	if p.PacketLoss >= 0 && p.LatencyMs < 50 && p.JitterMs < 15 && p.PacketLoss < 1 {
		s = append(s, "gaming")
	}

	if p.DownloadMbps >= 50 || p.UploadMbps >= 50 {
		s = append(s, "large_transfers")
	}

	return s
}

func concerns(p Params) []string {
	c := []string{}

	if p.LatencyMs > 100 {
		c = append(c, "high_latency")
	}
	if p.JitterMs > 30 {
		c = append(c, "high_jitter")
	}
	if p.PacketLoss >= 0 && p.PacketLoss > 1 {
		c = append(c, "packet_loss")
	}
	if p.DownloadMbps > 0 && p.DownloadMbps < 5 {
		c = append(c, "slow_download")
	}
	if p.UploadMbps > 0 && p.UploadMbps < 2 {
		c = append(c, "slow_upload")
	}

	return c
}

func buildSummary(grade string, p Params) string {
	gradeDesc := map[string]string{
		"A": "Excellent",
		"B": "Good",
		"C": "Fair",
		"D": "Poor",
		"F": "Very poor",
	}

	desc := gradeDesc[grade]

	parts := []string{}
	if p.DownloadMbps > 0 {
		parts = append(parts, fmt.Sprintf("%.0f Mbps down", p.DownloadMbps))
	}
	if p.UploadMbps > 0 {
		parts = append(parts, fmt.Sprintf("%.0f Mbps up", p.UploadMbps))
	}
	if p.LatencyMs > 0 {
		parts = append(parts, fmt.Sprintf("%.0fms latency", p.LatencyMs))
	}

	var summary strings.Builder
	summary.WriteString(desc + " connection")
	if len(parts) > 0 {
		summary.WriteString(": ")
		for i, part := range parts {
			if i > 0 {
				summary.WriteString(", ")
			}
			summary.WriteString(part)
		}
	}

	return summary.String()
}
