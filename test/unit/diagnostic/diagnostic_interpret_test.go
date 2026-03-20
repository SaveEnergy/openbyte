package diagnostic_test

import (
	"testing"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// --- Grade computation ---

func TestGradeAExcellentAll(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, UploadMbps: 100, LatencyMs: 5, JitterMs: 1,
	})
	if interp.Grade != "A" {
		t.Errorf("expected A, got %s", interp.Grade)
	}
}

func TestGradeBGoodConnection(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 50, UploadMbps: 10, LatencyMs: 30, JitterMs: 5,
	})
	if interp.Grade != "B" {
		t.Errorf("expected B, got %s (latency=%s speed=%s stab=%s)",
			interp.Grade, interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	}
}

func TestGradeCFairConnection(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 10, LatencyMs: 80, JitterMs: 20,
	})
	if interp.Grade != "C" {
		t.Errorf("expected C, got %s", interp.Grade)
	}
}

func TestGradeDPoorConnection(t *testing.T) {
	// Score: fair(2) + slow(0) + fair(2) = 4 → D
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 3, LatencyMs: 75, JitterMs: 15,
	})
	if interp.Grade != "D" {
		t.Errorf("expected D, got %s (latency=%s speed=%s stab=%s)",
			interp.Grade, interp.LatencyRating, interp.SpeedRating, interp.StabilityRating)
	}
}

func TestGradeFVeryPoor(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 1, LatencyMs: 500, PacketLoss: 10,
	})
	if interp.Grade != "F" {
		t.Errorf("expected F, got %s", interp.Grade)
	}
}

// --- Suitability ---

func TestSuitabilityAllWorkloads(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, UploadMbps: 100, LatencyMs: 5, JitterMs: 1,
	})
	expected := map[string]bool{
		useWebBrowsing:       true,
		useVideoConferencing: true,
		useStreaming4K:       true,
		useGaming:            true,
		useLargeTransfers:    true,
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

func TestSuitabilityBrowsingOnly(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 2, LatencyMs: 50, JitterMs: 20,
	})
	found := false
	for _, s := range interp.SuitableFor {
		if s == useWebBrowsing {
			found = true
		}
		if s == useVideoConferencing || s == useStreaming4K || s == useGaming {
			t.Errorf("unexpected suitability %s for slow connection", s)
		}
	}
	if !found {
		t.Error("expected web_browsing in suitable_for")
	}
}

func TestSuitabilityStreamingHD(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 15, LatencyMs: 30,
	})
	found := false
	for _, s := range interp.SuitableFor {
		if s == useStreamingHD {
			found = true
		}
		if s == useStreaming4K {
			t.Error("should not include streaming_4k for 15 Mbps")
		}
	}
	if !found {
		t.Error("expected streaming_hd in suitable_for")
	}
}

func TestSuitabilityNoGamingHighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, LatencyMs: 80, JitterMs: 5,
	})
	for _, s := range interp.SuitableFor {
		if s == useGaming {
			t.Error("should not include gaming with 80ms latency")
		}
	}
}

func TestSuitabilityNoGamingHighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 500, LatencyMs: 10, JitterMs: 20,
	})
	for _, s := range interp.SuitableFor {
		if s == useGaming {
			t.Error("should not include gaming with 20ms jitter")
		}
	}
}

func TestSuitabilityNoBrowsingHighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 250,
	})
	for _, s := range interp.SuitableFor {
		if s == useWebBrowsing {
			t.Error("should not include browsing with 250ms latency")
		}
	}
}

func TestSuitabilityEmptyHighLatency(t *testing.T) {
	// With 200ms+ latency and no speed, nothing qualifies
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 250})
	if len(interp.SuitableFor) != 0 {
		t.Errorf("expected empty suitable_for with high latency & no speed, got %v", interp.SuitableFor)
	}
}

func TestSuitabilityGamingZeroMetrics(t *testing.T) {
	// Gaming qualifies with all-zero metrics (0 < 50, 0 < 15, 0 < 1)
	interp := diagnostic.Interpret(diagnostic.Params{LatencyMs: 10})
	found := false
	for _, s := range interp.SuitableFor {
		if s == useGaming {
			found = true
		}
	}
	if !found {
		t.Error("expected gaming in suitable_for when latency/jitter/loss all low")
	}
}

// --- Concerns ---

func TestConcernsNoConcerns(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, UploadMbps: 50, LatencyMs: 10, JitterMs: 2,
	})
	if len(interp.Concerns) != 0 {
		t.Errorf("expected no concerns, got %v", interp.Concerns)
	}
}

func TestConcernsHighLatency(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 150,
	})
	assertContains(t, interp.Concerns, concernHighLatency)
}

func TestConcernsHighJitter(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 10, JitterMs: 50,
	})
	assertContains(t, interp.Concerns, concernHighJitter)
}

func TestConcernsPacketLoss(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, LatencyMs: 10, PacketLoss: 3,
	})
	assertContains(t, interp.Concerns, concernPacketLoss)
}

func TestConcernsSlowDownload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 3, LatencyMs: 10,
	})
	assertContains(t, interp.Concerns, concernSlowDown)
}

func TestConcernsSlowUpload(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		UploadMbps: 1, LatencyMs: 10,
	})
	assertContains(t, interp.Concerns, concernSlowUp)
}

func TestConcernsMultiple(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 2, UploadMbps: 1, LatencyMs: 200, JitterMs: 50, PacketLoss: 3,
	})
	assertContains(t, interp.Concerns, concernHighLatency)
	assertContains(t, interp.Concerns, concernHighJitter)
	assertContains(t, interp.Concerns, concernPacketLoss)
	assertContains(t, interp.Concerns, concernSlowDown)
	assertContains(t, interp.Concerns, concernSlowUp)
}

// --- Summary ---

func TestSummaryIncludesGradeDesc(t *testing.T) {
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

func TestSummaryContainsMetrics(t *testing.T) {
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

func TestSummaryOmitsZeroMetrics(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{DownloadMbps: 100, LatencyMs: 10})
	if containsStr(interp.Summary, "up") {
		t.Errorf("summary should not contain upload when 0, got %s", interp.Summary)
	}
}

// --- Interpret non-nil fields ---

func TestInterpretSuitableForNeverNil(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{})
	if interp.SuitableFor == nil {
		t.Error("SuitableFor should never be nil")
	}
}

func TestInterpretConcernsNeverNil(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{})
	if interp.Concerns == nil {
		t.Error("Concerns should never be nil")
	}
}
