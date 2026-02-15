package check

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

func TestIsValidServerURLRejectsInvalidPorts(t *testing.T) {
	if isValidServerURL("https://example.com:99999") {
		t.Fatal("expected invalid port to be rejected")
	}
	if !isValidServerURL("https://example.com:443") {
		t.Fatal("expected valid port to be accepted")
	}
}

func TestCheckRejectsInvalidFlagServerURL(t *testing.T) {
	code := Run([]string{"--server-url", "https://example.com:99999"}, "test")
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
}

func TestCheckDurationMsMatchesSDKMeasurement(t *testing.T) {
	origRunCheck := runCheckFn
	defer func() { runCheckFn = origRunCheck }()

	runCheckFn = func(_ context.Context, serverURL, _ string) (*CheckResult, error) {
		return &CheckResult{
			SchemaVersion: "1.0",
			Status:        "ok",
			ServerURL:     serverURL,
			LatencyMs:     10,
			DownloadMbps:  100,
			UploadMbps:    50,
			JitterMs:      1,
			DurationMs:    1234,
			Interpretation: &diagnostic.Interpretation{
				Grade:   "A",
				Summary: "ok",
			},
		}, nil
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := Run([]string{"--json", "--server-url", "https://example.com"}, "test")
	_ = w.Close()
	os.Stdout = oldStdout
	if code != exitSuccess {
		t.Fatalf("exit code = %d, want %d", code, exitSuccess)
	}

	var out CheckResult
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if out.DurationMs != 1234 {
		t.Fatalf("duration_ms = %d, want %d", out.DurationMs, 1234)
	}
}
