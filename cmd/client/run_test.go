package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/pkg/types"
)

type noopOutputFormatter struct{}

func (noopOutputFormatter) FormatProgress(progress, elapsed, remaining float64) {
	_ = progress
	_ = elapsed
	_ = remaining
}
func (noopOutputFormatter) FormatMetrics(metrics *types.Metrics) {
	_ = metrics
}
func (noopOutputFormatter) FormatComplete(results *StreamResults) {
	_ = results
}
func (noopOutputFormatter) FormatError(err error) {
	_ = err
}

func TestComputePingMetricsEmpty(t *testing.T) {
	latency, jitter := computePingMetrics(nil)
	if latency.Count != 0 {
		t.Fatalf("latency count = %d, want 0", latency.Count)
	}
	if jitter != 0 {
		t.Fatalf("jitter = %v, want 0", jitter)
	}
}

func TestComputePingMetricsPopulated(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
	}
	latency, jitter := computePingMetrics(samples)
	if latency.Count != 3 {
		t.Fatalf("latency count = %d, want 3", latency.Count)
	}
	if latency.MinMs != 10 || latency.MaxMs != 30 {
		t.Fatalf("latency min/max = %v/%v, want 10/30", latency.MinMs, latency.MaxMs)
	}
	if jitter <= 0 {
		t.Fatalf("jitter = %v, want > 0", jitter)
	}
}

func TestBuildResultsDirectionThroughput(t *testing.T) {
	cfg := &Config{
		Duration:  10,
		Streams:   2,
		ChunkSize: 1024 * 1024,
	}
	metrics := EngineMetrics{
		ThroughputMbps:   120,
		BytesTransferred: 1024,
	}

	cases := []struct {
		name      string
		direction string
		wantDown  float64
	}{
		{name: "download", direction: "download", wantDown: 120},
		{name: "upload", direction: "upload", wantDown: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Direction = tc.direction
			got := buildResults("stream-id", cfg, metrics, time.Now().Add(-1*time.Second))
			assertBuildResultsDirection(t, got, tc.direction, cfg.ChunkSize, tc.wantDown)
		})
	}
}

func assertBuildResultsDirection(t *testing.T, got *StreamResults, direction string, chunkSize int, wantDown float64) {
	t.Helper()
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q, want %q", got.SchemaVersion, SchemaVersion)
	}
	if got.Config == nil || got.Results == nil || got.Interpretation == nil {
		t.Fatal("expected populated config/results/interpretation")
	}
	if got.Config.Direction != direction {
		t.Fatalf("direction = %q, want %q", got.Config.Direction, direction)
	}
	if got.Config.Protocol != protocolHTTP {
		t.Fatalf("protocol = %q, want %q", got.Config.Protocol, protocolHTTP)
	}
	if got.Config.ChunkSize != chunkSize {
		t.Fatalf("chunk size = %d, want %d", got.Config.ChunkSize, chunkSize)
	}
	got4k := contains(got.Interpretation.SuitableFor, "streaming_4k")
	want4k := wantDown >= 25
	if got4k != want4k {
		t.Fatalf("streaming_4k suitability = %v, want %v", got4k, want4k)
	}
}

func TestCreateFormatterSelection(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "json", cfg: Config{JSON: true}, want: "*client.JSONFormatter"},
		{name: "ndjson", cfg: Config{NDJSON: true}, want: "*client.NDJSONFormatter"},
		{name: "plain", cfg: Config{Plain: true}, want: "*client.PlainFormatter"},
		{name: "interactive-default", cfg: Config{}, want: "*client.InteractiveFormatter"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := createFormatter(&tc.cfg)
			got := typeName(f)
			if got != tc.want {
				t.Fatalf("formatter type = %q, want %q", got, tc.want)
			}
		})
	}
}

type classifierTimeoutErr struct{}

func (classifierTimeoutErr) Error() string   { return "timeout" }
func (classifierTimeoutErr) Timeout() bool   { return true }
func (classifierTimeoutErr) Temporary() bool { return true }

func TestClassifyErrorCode(t *testing.T) {
	dialErr := &net.OpError{Op: "dial", Err: errors.New("dial failed")}
	netTimeoutErr := &net.OpError{Op: "read", Err: classifierTimeoutErr{}}

	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", err: nil, want: "unknown"},
		{name: "context canceled", err: context.Canceled, want: "cancelled"},
		{name: "context deadline", err: context.DeadlineExceeded, want: "timeout"},
		{name: "net dial", err: dialErr, want: "connection_refused"},
		{name: "net timeout", err: netTimeoutErr, want: "timeout"},
		{name: "message no such host", err: errors.New("lookup x: no such host"), want: "server_unavailable"},
		{name: "message rate limited", err: errors.New("request failed with 429"), want: "rate_limited"},
		{name: "message invalid", err: errors.New("invalid value"), want: "invalid_config"},
		{name: "fallback unknown", err: errors.New("unexpected failure"), want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyErrorCode(tc.err); got != tc.want {
				t.Fatalf("classifyErrorCode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func typeName(v any) string {
	return fmt.Sprintf("%T", v)
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}

type formatterWithErr struct {
	err error
}

func (f *formatterWithErr) FormatProgress(progress float64, elapsed, remaining float64) {
	_ = progress
	_ = elapsed
	_ = remaining
}
func (f *formatterWithErr) FormatMetrics(metrics *types.Metrics) {
	_ = metrics
}
func (f *formatterWithErr) FormatComplete(results *StreamResults) {
	_ = results
}
func (f *formatterWithErr) FormatError(err error) {
	_ = err
}
func (f *formatterWithErr) LastError() error { return f.err }

func TestFormatterLastError(t *testing.T) {
	want := errors.New("writer failed")
	f := &formatterWithErr{err: want}
	if got := formatterLastError(f); !errors.Is(got, want) {
		t.Fatalf("formatterLastError = %v, want %v", got, want)
	}
}

func TestHTTPTimeoutAtLeastDuration(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/download", handler.Download)
	mux.HandleFunc("/api/v1/ping", handler.Ping)
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &Config{
		ServerURL:  server.URL,
		Direction:  "download",
		Duration:   2,
		Streams:    1,
		ChunkSize:  65536,
		Timeout:    1,
		WarmUp:     0,
		NoProgress: true,
		Plain:      true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := runHTTPStream(ctx, cfg, noopOutputFormatter{}); err != nil {
		t.Fatalf("runHTTPStream failed with short timeout config: %v", err)
	}
}
