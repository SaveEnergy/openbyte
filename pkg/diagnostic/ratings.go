package diagnostic

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
	"unknown":   2,
}

func computeGrade(latency, speed, stability string) string {
	score := ratingScore[latency] + ratingScore[speed] + ratingScore[stability]
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
