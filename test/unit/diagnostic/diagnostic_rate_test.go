package diagnostic_test

import (
	"testing"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// --- rateLatency ---

func TestRateLatencyExcellent(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10, DownloadMbps: 100})
	if interp.LatencyRating != ratingExcellent {
		t.Errorf("expected excellent, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyGood(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 35, DownloadMbps: 100})
	if interp.LatencyRating != ratingGood {
		t.Errorf("expected good, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyFair(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 75, DownloadMbps: 100})
	if interp.LatencyRating != ratingFair {
		t.Errorf("expected fair, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyPoor(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 200, DownloadMbps: 100})
	if interp.LatencyRating != ratingPoor {
		t.Errorf("expected poor, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyUnknownZero(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 0, DownloadMbps: 100})
	if interp.LatencyRating != ratingUnknown {
		t.Errorf("expected unknown for 0ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyUnknownNegative(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: -5, DownloadMbps: 100})
	if interp.LatencyRating != ratingUnknown {
		t.Errorf("expected unknown for negative, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyBoundary20(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 20, DownloadMbps: 100})
	if interp.LatencyRating != ratingExcellent {
		t.Errorf("expected excellent at boundary 20ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyBoundary50(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 50, DownloadMbps: 100})
	if interp.LatencyRating != ratingGood {
		t.Errorf("expected good at boundary 50ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatencyBoundary100(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 100, DownloadMbps: 100})
	if interp.LatencyRating != ratingFair {
		t.Errorf("expected fair at boundary 100ms, got %s", interp.LatencyRating)
	}
}

// --- rateSpeed ---

func TestRateSpeedFast(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 500, LatencyMs: 10})
	if interp.SpeedRating != ratingFast {
		t.Errorf("expected fast, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedGood(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 50, LatencyMs: 10})
	if interp.SpeedRating != ratingGood {
		t.Errorf("expected good, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedModerate(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 10, LatencyMs: 10})
	if interp.SpeedRating != ratingModerate {
		t.Errorf("expected moderate, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedSlow(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 2, LatencyMs: 10})
	if interp.SpeedRating != ratingSlow {
		t.Errorf("expected slow, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedUnknown(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10})
	if interp.SpeedRating != ratingUnknown {
		t.Errorf("expected unknown, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedFallbackToUpload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{UploadMbps: 200, LatencyMs: 10})
	if interp.SpeedRating != ratingFast {
		t.Errorf("expected fast from upload fallback, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedBoundary100(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if interp.SpeedRating != ratingFast {
		t.Errorf("expected fast at boundary 100, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedBoundary25(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 25, LatencyMs: 10})
	if interp.SpeedRating != ratingGood {
		t.Errorf("expected good at boundary 25, got %s", interp.SpeedRating)
	}
}

func TestRateSpeedBoundary5(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 5, LatencyMs: 10})
	if interp.SpeedRating != ratingModerate {
		t.Errorf("expected moderate at boundary 5, got %s", interp.SpeedRating)
	}
}

// --- rateStability ---

func TestRateStabilityStableNoData(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if interp.StabilityRating != ratingStable {
		t.Errorf("expected stable with no jitter/loss data, got %s", interp.StabilityRating)
	}
}

func TestRateStabilityStableLowJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 5})
	if interp.StabilityRating != ratingStable {
		t.Errorf("expected stable, got %s", interp.StabilityRating)
	}
}

func TestRateStabilityFair(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 20})
	if interp.StabilityRating != ratingFair {
		t.Errorf("expected fair, got %s", interp.StabilityRating)
	}
}

func TestRateStabilityDegradedHighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 35})
	if interp.StabilityRating != ratingDegraded {
		t.Errorf("expected degraded, got %s", interp.StabilityRating)
	}
}

func TestRateStabilityDegradedPacketLoss(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, PacketLoss: 1.0})
	if interp.StabilityRating != ratingDegraded {
		t.Errorf("expected degraded, got %s", interp.StabilityRating)
	}
}

func TestRateStabilityUnstable(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, PacketLoss: 5})
	if interp.StabilityRating != ratingUnstable {
		t.Errorf("expected unstable, got %s", interp.StabilityRating)
	}
}
