package diagnostic_test

import (
	"testing"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// --- rateLatency ---

func TestRateLatency_Excellent(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10, DownloadMbps: 100})
	if interp.LatencyRating != "excellent" {
		t.Errorf("expected excellent, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Good(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 35, DownloadMbps: 100})
	if interp.LatencyRating != "good" {
		t.Errorf("expected good, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Fair(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 75, DownloadMbps: 100})
	if interp.LatencyRating != "fair" {
		t.Errorf("expected fair, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Poor(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 200, DownloadMbps: 100})
	if interp.LatencyRating != "poor" {
		t.Errorf("expected poor, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Unknown_Zero(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 0, DownloadMbps: 100})
	if interp.LatencyRating != "unknown" {
		t.Errorf("expected unknown for 0ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Unknown_Negative(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: -5, DownloadMbps: 100})
	if interp.LatencyRating != "unknown" {
		t.Errorf("expected unknown for negative, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Boundary_20(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 20, DownloadMbps: 100})
	if interp.LatencyRating != "excellent" {
		t.Errorf("expected excellent at boundary 20ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Boundary_50(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 50, DownloadMbps: 100})
	if interp.LatencyRating != "good" {
		t.Errorf("expected good at boundary 50ms, got %s", interp.LatencyRating)
	}
}

func TestRateLatency_Boundary_100(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 100, DownloadMbps: 100})
	if interp.LatencyRating != "fair" {
		t.Errorf("expected fair at boundary 100ms, got %s", interp.LatencyRating)
	}
}

// --- rateSpeed ---

func TestRateSpeed_Fast(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 500, LatencyMs: 10})
	if interp.SpeedRating != "fast" {
		t.Errorf("expected fast, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Good(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 50, LatencyMs: 10})
	if interp.SpeedRating != "good" {
		t.Errorf("expected good, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Moderate(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 10, LatencyMs: 10})
	if interp.SpeedRating != "moderate" {
		t.Errorf("expected moderate, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Slow(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 2, LatencyMs: 10})
	if interp.SpeedRating != "slow" {
		t.Errorf("expected slow, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Unknown(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10})
	if interp.SpeedRating != "unknown" {
		t.Errorf("expected unknown, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_FallbackToUpload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{UploadMbps: 200, LatencyMs: 10})
	if interp.SpeedRating != "fast" {
		t.Errorf("expected fast from upload fallback, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Boundary_100(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if interp.SpeedRating != "fast" {
		t.Errorf("expected fast at boundary 100, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Boundary_25(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 25, LatencyMs: 10})
	if interp.SpeedRating != "good" {
		t.Errorf("expected good at boundary 25, got %s", interp.SpeedRating)
	}
}

func TestRateSpeed_Boundary_5(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 5, LatencyMs: 10})
	if interp.SpeedRating != "moderate" {
		t.Errorf("expected moderate at boundary 5, got %s", interp.SpeedRating)
	}
}

// --- rateStability ---

func TestRateStability_Stable_NoData(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if interp.StabilityRating != "stable" {
		t.Errorf("expected stable with no jitter/loss data, got %s", interp.StabilityRating)
	}
}

func TestRateStability_Stable_LowJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 5})
	if interp.StabilityRating != "stable" {
		t.Errorf("expected stable, got %s", interp.StabilityRating)
	}
}

func TestRateStability_Fair(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 20})
	if interp.StabilityRating != "fair" {
		t.Errorf("expected fair, got %s", interp.StabilityRating)
	}
}

func TestRateStability_Degraded_HighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, JitterMs: 35})
	if interp.StabilityRating != "degraded" {
		t.Errorf("expected degraded, got %s", interp.StabilityRating)
	}
}

func TestRateStability_Degraded_PacketLoss(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, PacketLoss: 1.0})
	if interp.StabilityRating != "degraded" {
		t.Errorf("expected degraded, got %s", interp.StabilityRating)
	}
}

func TestRateStability_Unstable(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10, PacketLoss: 5})
	if interp.StabilityRating != "unstable" {
		t.Errorf("expected unstable, got %s", interp.StabilityRating)
	}
}

// --- Grade computation ---

func TestGrade_A_ExcellentAll(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, UploadMbps: 100, LatencyMs: 5, JitterMs: 1,
	})
	if interp.Grade != "A" {
		t.Errorf("expected A, got %s", interp.Grade)
	}
}

func TestGrade_B_GoodConnection(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 50, UploadMbps: 10, LatencyMs: 30, JitterMs: 5,
	})
	if interp.Grade != "B" {
		t.Errorf("expected B, got %s (latency=%s speed=%s stab=%s)",
			interp.Grade, interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	}
}

func TestGrade_C_FairConnection(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 10, LatencyMs: 80, JitterMs: 20,
	})
	if interp.Grade != "C" {
		t.Errorf("expected C, got %s", interp.Grade)
	}
}

func TestGrade_D_PoorConnection(t *testing.T) {
	// Score: fair(2) + slow(0) + fair(2) = 4 → D
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 3, LatencyMs: 75, JitterMs: 15,
	})
	if interp.Grade != "D" {
		t.Errorf("expected D, got %s (latency=%s speed=%s stab=%s)",
			interp.Grade, interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	}
}

func TestGrade_F_VeryPoor(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 1, LatencyMs: 500, PacketLoss: 10,
	})
	if interp.Grade != "F" {
		t.Errorf("expected F, got %s", interp.Grade)
	}
}

// --- Suitability ---

func TestSuitability_AllWorkloads(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, UploadMbps: 100, LatencyMs: 5, JitterMs: 1,
	})
	expected := map[string]bool{
		"web_browsing":       true,
		"video_conferencing": true,
		"streaming_4k":       true,
		"gaming":             true,
		"large_transfers":    true,
	}
	got := make(map[string]bool)
	for _, s := range interp.SuitableFor {
		got[s] = true
	}
	for k := range expected {
		if !got[k] {
			t.Errorf("expected %s in suitable_for, got %v", k, interp.SuitableFor)
		}
	}
}

func TestSuitability_BrowsingOnly(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 2, LatencyMs: 50, JitterMs: 20,
	})
	found := false
	for _, s := range interp.SuitableFor {
		if s == "web_browsing" {
			found = true
		}
		if s == "video_conferencing" || s == "streaming_4k" || s == "gaming" {
			t.Errorf("unexpected suitability %s for slow connection", s)
		}
	}
	if !found {
		t.Error("expected web_browsing in suitable_for")
	}
}

func TestSuitability_StreamingHD(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 15, LatencyMs: 30,
	})
	found := false
	for _, s := range interp.SuitableFor {
		if s == "streaming_hd" {
			found = true
		}
		if s == "streaming_4k" {
			t.Error("should not include streaming_4k for 15 Mbps")
		}
	}
	if !found {
		t.Error("expected streaming_hd in suitable_for")
	}
}

func TestSuitability_NoGaming_HighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, LatencyMs: 80, JitterMs: 5,
	})
	for _, s := range interp.SuitableFor {
		if s == "gaming" {
			t.Error("should not include gaming with 80ms latency")
		}
	}
}

func TestSuitability_NoGaming_HighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, LatencyMs: 10, JitterMs: 20,
	})
	for _, s := range interp.SuitableFor {
		if s == "gaming" {
			t.Error("should not include gaming with 20ms jitter")
		}
	}
}

func TestSuitability_NoBrowsing_HighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 250,
	})
	for _, s := range interp.SuitableFor {
		if s == "web_browsing" {
			t.Error("should not include browsing with 250ms latency")
		}
	}
}

func TestSuitability_Empty_HighLatency(t *testing.T) {
	// With 200ms+ latency and no speed, nothing qualifies
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 250})
	if len(interp.SuitableFor) != 0 {
		t.Errorf("expected empty suitable_for with high latency & no speed, got %v", interp.SuitableFor)
	}
}

func TestSuitability_Gaming_ZeroMetrics(t *testing.T) {
	// Gaming qualifies with all-zero metrics (0 < 50, 0 < 15, 0 < 1)
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10})
	found := false
	for _, s := range interp.SuitableFor {
		if s == "gaming" {
			found = true
		}
	}
	if !found {
		t.Error("expected gaming in suitable_for when latency/jitter/loss all low")
	}
}

// --- Concerns ---

func TestConcerns_NoConcerns(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, UploadMbps: 50, LatencyMs: 10, JitterMs: 2,
	})
	if len(interp.Concerns) != 0 {
		t.Errorf("expected no concerns, got %v", interp.Concerns)
	}
}

func TestConcerns_HighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 150,
	})
	assertContains(t, interp.Concerns, "high_latency")
}

func TestConcerns_HighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 10, JitterMs: 50,
	})
	assertContains(t, interp.Concerns, "high_jitter")
}

func TestConcerns_PacketLoss(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 10, PacketLoss: 3,
	})
	assertContains(t, interp.Concerns, "packet_loss")
}

func TestConcerns_SlowDownload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 3, LatencyMs: 10,
	})
	assertContains(t, interp.Concerns, "slow_download")
}

func TestConcerns_SlowUpload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		UploadMbps: 1, LatencyMs: 10,
	})
	assertContains(t, interp.Concerns, "slow_upload")
}

func TestConcerns_Multiple(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 2, UploadMbps: 1, LatencyMs: 200, JitterMs: 50, PacketLoss: 3,
	})
	assertContains(t, interp.Concerns, "high_latency")
	assertContains(t, interp.Concerns, "high_jitter")
	assertContains(t, interp.Concerns, "packet_loss")
	assertContains(t, interp.Concerns, "slow_download")
	assertContains(t, interp.Concerns, "slow_upload")
}

// --- Summary ---

func TestSummary_IncludesGradeDesc(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, LatencyMs: 5,
	})
	if interp.Summary == "" {
		t.Fatal("summary should not be empty")
	}
	if interp.Grade == "A" && !containsStr(interp.Summary, "Excellent") {
		t.Errorf("grade A summary should contain 'Excellent', got %s", interp.Summary)
	}
}

func TestSummary_ContainsMetrics(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 450, UploadMbps: 50, LatencyMs: 12,
	})
	if !containsStr(interp.Summary, "450 Mbps down") {
		t.Errorf("summary should contain download speed, got %s", interp.Summary)
	}
	if !containsStr(interp.Summary, "50 Mbps up") {
		t.Errorf("summary should contain upload speed, got %s", interp.Summary)
	}
	if !containsStr(interp.Summary, "12ms latency") {
		t.Errorf("summary should contain latency, got %s", interp.Summary)
	}
}

func TestSummary_OmitsZeroMetrics(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if containsStr(interp.Summary, "up") {
		t.Errorf("summary should not contain upload when 0, got %s", interp.Summary)
	}
}

// --- Interpret non-nil fields ---

func TestInterpret_SuitableForNeverNil(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{})
	if interp.SuitableFor == nil {
		t.Error("SuitableFor should never be nil")
	}
}

func TestInterpret_ConcernsNeverNil(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{})
	if interp.Concerns == nil {
		t.Error("Concerns should never be nil")
	}
}

// --- Helpers ---

func assertContains(t *testing.T, slice []string, item string) {
	t.Helper()
	for _, s := range slice {
		if s == item {
			return
		}
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
