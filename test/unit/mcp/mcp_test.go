package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cmdmcp "github.com/saveenergy/openbyte/cmd/mcp"
	"github.com/saveenergy/openbyte/internal/api"
	pkgclient "github.com/saveenergy/openbyte/pkg/client"
	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// newMCPTestServer creates a httptest server with health, ping, download, upload endpoints.
func newMCPTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	handler := api.NewSpeedTestHandler(10, 300)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/v1/ping", handler.Ping)
	mux.HandleFunc("GET /api/v1/download", handler.Download)
	mux.HandleFunc("POST /api/v1/upload", handler.Upload)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// The MCP handlers are internal to cmd/mcp, but we can test the underlying
// logic via the pkg/client SDK (which the MCP handlers should use in production).
// These tests validate that the SDK produces results suitable for MCP tool responses.

func TestMCP_ConnectivityCheck_ViaSDK(t *testing.T) {
	srv := newMCPTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("connectivity check failed: %v", err)
	}

	// Simulate what MCP handler does: marshal to JSON
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Verify it's valid JSON parseable by agents
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Required fields for agents
	for _, key := range []string{"status", "latency_ms", "download_mbps", "upload_mbps", "interpretation"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required field %q in MCP response", key)
		}
	}
}

func TestMCP_SpeedTest_ViaSDK(t *testing.T) {
	srv := newMCPTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := c.SpeedTest(ctx, pkgclient.SpeedTestOptions{
		Direction: "download",
		Duration:  1,
	})
	if err != nil {
		t.Fatalf("speed test failed: %v", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	for _, key := range []string{"direction", "throughput_mbps", "latency_ms", "interpretation"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required field %q in MCP speed test response", key)
		}
	}
}

func TestMCP_Diagnose_ViaSDK(t *testing.T) {
	srv := newMCPTestServer(t)
	c := pkgclient.New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Run both download and upload like the diagnose tool does
	checkResult, err := c.Check(ctx)
	if err != nil {
		t.Fatalf("diagnose failed: %v", err)
	}

	// Verify interpretation has all fields needed for diagnosis
	interp := checkResult.Interpretation
	if interp == nil {
		t.Fatal("interpretation should not be nil")
	}

	// All ratings should be populated
	if interp.LatencyRating == "" {
		t.Error("latency_rating should not be empty")
	}
	if interp.SpeedRating == "" {
		t.Error("speed_rating should not be empty")
	}
	if interp.StabilityRating == "" {
		t.Error("stability_rating should not be empty")
	}
}

func TestMCP_ErrorResult_AgentReadable(t *testing.T) {
	// Simulate what MCP handler does on error: return tool error text
	c := pkgclient.New("http://127.0.0.1:1")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := c.Check(ctx)
	if err == nil {
		t.Fatal("expected error")
	}

	// MCP handler wraps: fmt.Sprintf("Connectivity check failed: %v", err)
	errMsg := "Connectivity check failed: " + err.Error()
	if errMsg == "" {
		t.Error("error message should not be empty")
	}
}

// --- Diagnostic integration tests ---

func TestDiagnostic_InterpretAllGrades(t *testing.T) {
	tests := []struct {
		name  string
		p     diagnostic.Params
		grade string
	}{
		{"A grade", diagnostic.Params{DownloadMbps: 500, UploadMbps: 100, LatencyMs: 5, JitterMs: 1}, "A"},
		{"B grade", diagnostic.Params{DownloadMbps: 50, UploadMbps: 10, LatencyMs: 30, JitterMs: 5}, "B"},
		{"C grade", diagnostic.Params{DownloadMbps: 10, LatencyMs: 80, JitterMs: 20}, "C"},
		{"D grade", diagnostic.Params{DownloadMbps: 3, LatencyMs: 75, JitterMs: 15}, "D"},
		{"F grade", diagnostic.Params{DownloadMbps: 1, LatencyMs: 500, PacketLoss: 10}, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interp := diagnostic.Interpret(tt.p)
			if interp.Grade != tt.grade {
				t.Errorf("expected grade %s, got %s", tt.grade, interp.Grade)
			}
		})
	}
}

func TestDiagnostic_JSONSchema(t *testing.T) {
	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: 100, UploadMbps: 50, LatencyMs: 10, JitterMs: 2,
	})

	data, err := json.Marshal(interp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	expectedKeys := []string{
		"grade", "summary", "latency_rating", "speed_rating",
		"stability_rating", "suitable_for", "concerns",
	}
	for _, key := range expectedKeys {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing key %q in interpretation JSON", key)
		}
	}
}

func TestValidateServerURL(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		valid bool
	}{
		{name: "http", raw: "http://localhost:8080", valid: true},
		{name: "https", raw: "https://speed.example.com", valid: true},
		{name: "missing scheme", raw: "localhost:8080", valid: false},
		{name: "unsupported scheme", raw: "ftp://example.com", valid: false},
		{name: "missing host", raw: "http://", valid: false},
		{name: "query not allowed", raw: "https://speed.example.com?x=1", valid: false},
		{name: "fragment not allowed", raw: "https://speed.example.com#frag", valid: false},
		{name: "port too high", raw: "https://speed.example.com:65536", valid: false},
		{name: "port zero", raw: "https://speed.example.com:0", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cmdmcp.ValidateServerURL(tt.raw)
			if tt.valid && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateSpeedTestInput(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		duration  int
		valid     bool
	}{
		{name: "valid download", direction: "download", duration: 10, valid: true},
		{name: "valid upload", direction: "upload", duration: 1, valid: true},
		{name: "invalid direction", direction: "invalid", duration: 10, valid: false},
		{name: "too small duration", direction: "download", duration: 0, valid: false},
		{name: "too large duration", direction: "upload", duration: 61, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmdmcp.ValidateSpeedTestInput(tt.direction, tt.duration)
			if tt.valid && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestMCPToolContract(t *testing.T) {
	tools := cmdmcp.ToolDefinitions()
	if len(tools) != 3 {
		t.Fatalf("tool count = %d, want 3", len(tools))
	}

	byName := make(map[string]bool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = true
		props := tool.InputSchema.Properties
		if _, ok := props["server_url"]; !ok {
			t.Fatalf("%s missing server_url argument", tool.Name)
		}
		if _, ok := props["api_key"]; !ok {
			t.Fatalf("%s missing api_key argument", tool.Name)
		}
	}

	for _, name := range []string{"connectivity_check", "speed_test", "diagnose"} {
		if !byName[name] {
			t.Fatalf("missing tool definition %q", name)
		}
	}
}
