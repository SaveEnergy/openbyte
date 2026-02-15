// Package diagnostic interprets raw speed test metrics into human/agent-readable
// grades, ratings, and suitability assessments.
package diagnostic

import "fmt"

// Interpretation holds the semantic interpretation of speed test results.
type Interpretation struct {
	Grade           string   `json:"grade"`
	Summary         string   `json:"summary"`
	LatencyRating   string   `json:"latency_rating"`
	SpeedRating     string   `json:"speed_rating"`
	StabilityRating string   `json:"stability_rating"`
	SuitableFor     []string `json:"suitable_for"`
	Concerns        []string `json:"concerns"`
}

// Params are the raw metrics to interpret.
type Params struct {
	DownloadMbps float64
	UploadMbps   float64
	LatencyMs    float64
	JitterMs     float64
	PacketLoss   float64
}

// Interpret produces a diagnostic Interpretation from raw metrics.
func Interpret(p Params) *Interpretation {
	interp := &Interpretation{
		SuitableFor: []string{},
		Concerns:    []string{},
	}

	interp.LatencyRating = rateLatency(p.LatencyMs)
	interp.SpeedRating = rateSpeed(p.DownloadMbps, p.UploadMbps)
	interp.StabilityRating = rateStability(p.JitterMs, p.PacketLoss)

	interp.SuitableFor = suitability(p)
	interp.Concerns = concerns(p)

	interp.Grade = computeGrade(interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	interp.Summary = buildSummary(interp.Grade, p)

	return interp
}

func rateLatency(ms float64) string {
	switch {
	case ms <= 0:
		return "unknown"
	case ms <= 20:
		return "excellent"
	case ms <= 50:
		return "good"
	case ms <= 100:
		return "fair"
	default:
		return "poor"
	}
}

func rateSpeed(downMbps, upMbps float64) string {
	// Use whichever is available; prefer download
	speed := downMbps
	if speed <= 0 {
		speed = upMbps
	}
	switch {
	case speed <= 0:
		return "unknown"
	case speed >= 100:
		return "fast"
	case speed >= 25:
		return "good"
	case speed >= 5:
		return "moderate"
	default:
		return "slow"
	}
}

func rateStability(jitterMs, packetLoss float64) string {
	if jitterMs <= 0 && packetLoss < 0 {
		return "unknown"
	}
	if packetLoss > 2 {
		return "unstable"
	}
	if packetLoss > 0.5 || jitterMs > 30 {
		return "degraded"
	}
	if jitterMs > 10 {
		return "fair"
	}
	return "stable"
}

func suitability(p Params) []string {
	s := []string{}

	// Browsing: 1+ Mbps, latency < 200ms
	if (p.DownloadMbps >= 1 || p.UploadMbps >= 1) && p.LatencyMs < 200 {
		s = append(s, "web_browsing")
	}

	// Video conferencing: 5+ Mbps down, 2+ up, latency < 100ms, jitter < 30ms
	if p.DownloadMbps >= 5 && p.UploadMbps >= 2 && p.LatencyMs < 100 && p.JitterMs < 30 {
		s = append(s, "video_conferencing")
	}

	// 4K streaming: 25+ Mbps down
	if p.DownloadMbps >= 25 {
		s = append(s, "streaming_4k")
	} else if p.DownloadMbps >= 5 {
		s = append(s, "streaming_hd")
	}

	// Gaming: latency < 50ms, jitter < 15ms, loss < 1%
	if p.PacketLoss >= 0 && p.LatencyMs < 50 && p.JitterMs < 15 && p.PacketLoss < 1 {
		s = append(s, "gaming")
	}

	// Large file transfers: 50+ Mbps
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

var ratingScore = map[string]int{
	"excellent": 4,
	"fast":      4,
	"stable":    4,
	"good":      3,
	"fair":      2,
	"moderate":  2,
	"degraded":  1,
	"poor":      0,
	"slow":      0,
	"unstable":  0,
	"unknown":   2, // neutral default
}

func computeGrade(latency, speed, stability string) string {
	score := ratingScore[latency] + ratingScore[speed] + ratingScore[stability]
	// Max score = 12 (4+4+4)
	switch {
	case score >= 11:
		return "A"
	case score >= 9:
		return "B"
	case score >= 6:
		return "C"
	case score >= 3:
		return "D"
	default:
		return "F"
	}
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

	summary := desc + " connection"
	if len(parts) > 0 {
		summary += ": "
		for i, part := range parts {
			if i > 0 {
				summary += ", "
			}
			summary += part
		}
	}

	return summary
}
