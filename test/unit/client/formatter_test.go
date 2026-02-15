package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"

	client "github.com/saveenergy/openbyte/cmd/client"
	"github.com/saveenergy/openbyte/pkg/types"
)

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}

// --- JSONFormatter.FormatError ---

func TestJSONFormatError_StructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(fmt.Errorf("connection refused"))

	var resp client.JSONErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON error: %v\nraw: %s", err, buf.String())
	}
	if !resp.Error {
		t.Error("error field should be true")
	}
	if resp.Code != "connection_refused" {
		t.Errorf("expected code connection_refused, got %s", resp.Code)
	}
	if resp.SchemaVersion != client.SchemaVersion {
		t.Errorf("expected schema_version %s, got %s", client.SchemaVersion, resp.SchemaVersion)
	}
}

func TestJSONFormatError_Timeout(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(context.DeadlineExceeded)

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "timeout" {
		t.Errorf("expected timeout, got %s", resp.Code)
	}
}

func TestJSONFormatError_Cancelled(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(context.Canceled)

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "cancelled" {
		t.Errorf("expected cancelled, got %s", resp.Code)
	}
}

func TestJSONFormatError_RateLimited(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(fmt.Errorf("server returned 429 too many requests"))

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "rate_limited" {
		t.Errorf("expected rate_limited, got %s", resp.Code)
	}
}

func TestJSONFormatError_ServerUnavailable(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(fmt.Errorf("no such host"))

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "server_unavailable" {
		t.Errorf("expected server_unavailable, got %s", resp.Code)
	}
}

func TestJSONFormatError_InvalidConfig(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(fmt.Errorf("invalid protocol: quic"))

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "invalid_config" {
		t.Errorf("expected invalid_config, got %s", resp.Code)
	}
}

func TestJSONFormatError_NetOpError_Dial(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	opErr := &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")}
	f.FormatError(opErr)

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "connection_refused" {
		t.Errorf("expected connection_refused for dial OpError, got %s", resp.Code)
	}
}

func TestJSONFormatError_Unknown(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	f.FormatError(fmt.Errorf("something unexpected happened"))

	var resp client.JSONErrorResponse
	json.Unmarshal(buf.Bytes(), &resp)
	if resp.Code != "unknown" {
		t.Errorf("expected unknown, got %s", resp.Code)
	}
}

// --- JSONFormatter.FormatComplete ---

func TestJSONFormatComplete_SchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	results := &client.StreamResults{
		SchemaVersion: client.SchemaVersion,
		StreamID:      "test-123",
		Status:        "completed",
	}
	f.FormatComplete(results)

	var parsed map[string]interface{}
	json.Unmarshal(buf.Bytes(), &parsed)
	if parsed["schema_version"] != client.SchemaVersion {
		t.Errorf("expected schema_version %s, got %v", client.SchemaVersion, parsed["schema_version"])
	}
}

func TestJSONFormatComplete_InterpretationIncluded(t *testing.T) {
	var buf bytes.Buffer
	f := &client.JSONFormatter{Writer: &buf}
	results := &client.StreamResults{
		SchemaVersion: client.SchemaVersion,
		StreamID:      "test-123",
		Status:        "completed",
	}
	f.FormatComplete(results)

	// Just verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

// --- NDJSONFormatter ---

func TestNDJSONFormatProgress(t *testing.T) {
	var buf bytes.Buffer
	f := &client.NDJSONFormatter{Writer: &buf}
	f.FormatProgress(50.0, 5.0, 5.0)

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid NDJSON progress: %v\nraw: %s", err, buf.String())
	}
	if parsed["type"] != "progress" {
		t.Errorf("expected type=progress, got %v", parsed["type"])
	}
	if parsed["percent"] != 50.0 {
		t.Errorf("expected percent=50, got %v", parsed["percent"])
	}
}

func TestNDJSONFormatMetrics(t *testing.T) {
	var buf bytes.Buffer
	f := &client.NDJSONFormatter{Writer: &buf}
	m := &types.Metrics{
		ThroughputMbps:   100.5,
		BytesTransferred: 1000000,
		Latency:          types.LatencyMetrics{AvgMs: 12.3},
		JitterMs:         1.5,
	}
	f.FormatMetrics(m)

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid NDJSON metrics: %v", err)
	}
	if parsed["type"] != "metrics" {
		t.Errorf("expected type=metrics, got %v", parsed["type"])
	}
	if parsed["throughput_mbps"] != 100.5 {
		t.Errorf("expected throughput_mbps=100.5, got %v", parsed["throughput_mbps"])
	}
}

func TestNDJSONFormatComplete(t *testing.T) {
	var buf bytes.Buffer
	f := &client.NDJSONFormatter{Writer: &buf}
	results := &client.StreamResults{
		SchemaVersion: client.SchemaVersion,
		StreamID:      "test-ndjson",
		Status:        "completed",
	}
	f.FormatComplete(results)

	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid NDJSON complete: %v", err)
	}
	if parsed["type"] != "result" {
		t.Errorf("expected type=result, got %v", parsed["type"])
	}
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field to be an object")
	}
	if data["stream_id"] != "test-ndjson" {
		t.Errorf("expected stream_id=test-ndjson, got %v", data["stream_id"])
	}
}

func TestNDJSONFormatError(t *testing.T) {
	var buf bytes.Buffer
	f := &client.NDJSONFormatter{Writer: &buf}
	f.FormatError(context.DeadlineExceeded)

	var resp client.JSONErrorResponse
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid NDJSON error: %v", err)
	}
	if !resp.Error {
		t.Error("error field should be true")
	}
	if resp.Code != "timeout" {
		t.Errorf("expected timeout, got %s", resp.Code)
	}
}

func TestNDJSONMultilineOutput(t *testing.T) {
	var buf bytes.Buffer
	f := &client.NDJSONFormatter{Writer: &buf}

	f.FormatProgress(25.0, 2.0, 6.0)
	f.FormatProgress(50.0, 4.0, 4.0)
	f.FormatComplete(&client.StreamResults{
		SchemaVersion: client.SchemaVersion,
		StreamID:      "multi",
		Status:        "completed",
	})

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Errorf("expected 3 NDJSON lines, got %d", len(lines))
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal(line, &parsed); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestNDJSONFormatterCapturesWriteError(t *testing.T) {
	f := &client.NDJSONFormatter{Writer: failingWriter{}}
	f.FormatProgress(10, 1, 9)
	if err := f.LastError(); err == nil {
		t.Fatal("expected LastError after failed write")
	}
}

// --- SchemaVersion constant ---

func TestSchemaVersion_Format(t *testing.T) {
	if client.SchemaVersion != "1.0" {
		t.Errorf("expected SchemaVersion 1.0, got %s", client.SchemaVersion)
	}
}
