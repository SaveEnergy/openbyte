package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type noopOutputFormatter struct{}

func (noopOutputFormatter) FormatProgress(float64, float64, float64) {}
func (noopOutputFormatter) FormatMetrics(*types.Metrics)             {}
func (noopOutputFormatter) FormatComplete(*StreamResults)            {}
func (noopOutputFormatter) FormatError(error)                        {}

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
		Protocol:   "tcp",
		Duration:   10,
		Streams:    2,
		PacketSize: 1400,
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
		{name: "bidirectional", direction: "bidirectional", wantDown: 60},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg.Direction = tc.direction
			got := buildResults("stream-id", cfg, metrics, time.Now().Add(-1*time.Second))
			if got.SchemaVersion != SchemaVersion {
				t.Fatalf("schema version = %q, want %q", got.SchemaVersion, SchemaVersion)
			}
			if got.Config == nil || got.Results == nil || got.Interpretation == nil {
				t.Fatal("expected populated config/results/interpretation")
			}
			if got.Config.Direction != tc.direction {
				t.Fatalf("direction = %q, want %q", got.Config.Direction, tc.direction)
			}
			got4k := contains(got.Interpretation.SuitableFor, "streaming_4k")
			want4k := tc.wantDown >= 25
			if got4k != want4k {
				t.Fatalf("streaming_4k suitability = %v, want %v", got4k, want4k)
			}
		})
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

func typeName(v interface{}) string {
	return fmt.Sprintf("%T", v)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type formatterWithErr struct {
	err error
}

func (f *formatterWithErr) FormatProgress(progress float64, elapsed, remaining float64) {}
func (f *formatterWithErr) FormatMetrics(metrics *types.Metrics)                        {}
func (f *formatterWithErr) FormatComplete(results *StreamResults)                       {}
func (f *formatterWithErr) FormatError(err error)                                       {}
func (f *formatterWithErr) LastError() error                                            { return f.err }

func TestFormatterLastError(t *testing.T) {
	want := errors.New("writer failed")
	f := &formatterWithErr{err: want}
	if got := formatterLastError(f); !errors.Is(got, want) {
		t.Fatalf("formatterLastError = %v, want %v", got, want)
	}
}

func TestRunCancelStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/stream/start":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"stream_id":"s1","websocket_url":"ws://127.0.0.1:1","status":"running","mode":"proxy"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/stream/s1/cancel":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &Config{
		Protocol:  "tcp",
		Direction: "download",
		Duration:  1,
		Streams:   1,
		ServerURL: server.URL,
		Timeout:   2,
	}

	var streamID atomic.Value
	err := runStream(context.Background(), cfg, noopOutputFormatter{}, &streamID)
	if err == nil {
		t.Fatal("expected runStream error")
	}
	if !strings.Contains(err.Error(), "cancel cleanup failed") {
		t.Fatalf("error = %q, want cancel cleanup context", err.Error())
	}
}
