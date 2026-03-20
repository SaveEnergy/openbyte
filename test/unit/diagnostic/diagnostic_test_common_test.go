package diagnostic_test

import (
	"slices"
	"testing"
)

const (
	ratingExcellent = "excellent"
	ratingGood      = "good"
	ratingFair      = "fair"
	ratingPoor      = "poor"
	ratingUnknown   = "unknown"
	ratingFast      = "fast"
	ratingModerate  = "moderate"
	ratingSlow      = "slow"
	ratingStable    = "stable"
	ratingDegraded  = "degraded"
	ratingUnstable  = "unstable"

	useWebBrowsing       = "web_browsing"
	useVideoConferencing = "video_conferencing"
	useStreamingHD       = "streaming_hd"
	useStreaming4K       = "streaming_4k"
	useGaming            = "gaming"
	useLargeTransfers    = "large_transfers"

	concernHighLatency = "high_latency"
	concernHighJitter  = "high_jitter"
	concernPacketLoss  = "packet_loss"
	concernSlowDown    = "slow_download"
	concernSlowUp      = "slow_upload"
)

func assertContains(t *testing.T, slice []string, item string) {
	t.Helper()
	if slices.Contains(slice, item) {
		return
	}
	t.Errorf("expected %v to contain %q", slice, item)
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
