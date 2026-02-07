// Package mcp implements the `openbyte mcp` subcommand — an MCP (Model Context
// Protocol) server over stdio transport. Agents can spawn this process and call
// connectivity tools directly.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/saveenergy/openbyte/pkg/diagnostic"
)

// Run starts the MCP stdio server. Blocks until stdin closes or signal received.
func Run(version string) int {
	s := server.NewMCPServer(
		"openbyte",
		version,
		server.WithToolCapabilities(true),
	)

	// Tool: connectivity_check — quick 3-5s connectivity test
	checkTool := mcp.NewTool("connectivity_check",
		mcp.WithDescription("Quick connectivity check (~3-5 seconds). Returns latency, rough download/upload speed, grade (A-F), and diagnostic interpretation. Use this for fast 'is the network OK?' checks."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
	)
	s.AddTool(checkTool, handleConnectivityCheck)

	// Tool: speed_test — full speed test
	speedTool := mcp.NewTool("speed_test",
		mcp.WithDescription("Full speed test with configurable duration. Returns detailed throughput, latency, jitter, and diagnostic interpretation. Use for accurate measurements."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
		mcp.WithString("direction",
			mcp.Description("Test direction: download or upload (default: download)"),
		),
		mcp.WithNumber("duration",
			mcp.Description("Test duration in seconds, 1-60 (default: 10)"),
		),
	)
	s.AddTool(speedTool, handleSpeedTest)

	// Tool: diagnose — latency + download + upload with full diagnostic
	diagnoseTool := mcp.NewTool("diagnose",
		mcp.WithDescription("Comprehensive network diagnosis: measures latency, download speed, upload speed, and returns bufferbloat grade, suitability assessment, and concerns. Takes ~15-20 seconds."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
	)
	s.AddTool(diagnoseTool, handleDiagnose)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "openbyte mcp: error: %v\n", err)
		return 1
	}
	return 0
}

// --- Tool Handlers ---

func handleConnectivityCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverURL := req.GetString("server_url", "http://localhost:8080")

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := quickCheck(checkCtx, serverURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Connectivity check failed: %v", err)), nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

func handleSpeedTest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverURL := req.GetString("server_url", "http://localhost:8080")
	direction := req.GetString("direction", "download")
	duration := req.GetInt("duration", 10)

	if duration < 1 {
		duration = 1
	}
	if duration > 60 {
		duration = 60
	}
	if direction != "download" && direction != "upload" {
		direction = "download"
	}

	testCtx, cancel := context.WithTimeout(ctx, time.Duration(duration+15)*time.Second)
	defer cancel()

	result, err := runSpeedTest(testCtx, serverURL, direction, duration)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Speed test failed: %v", err)), nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

func handleDiagnose(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverURL := req.GetString("server_url", "http://localhost:8080")

	diagCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := runDiagnosis(diagCtx, serverURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diagnosis failed: %v", err)), nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(data)), nil
}

// --- Internal implementations ---

type checkResult struct {
	Status         string                     `json:"status"`
	ServerURL      string                     `json:"server_url"`
	LatencyMs      float64                    `json:"latency_ms"`
	DownloadMbps   float64                    `json:"download_mbps"`
	UploadMbps     float64                    `json:"upload_mbps"`
	JitterMs       float64                    `json:"jitter_ms"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

type speedTestResult struct {
	Direction      string                     `json:"direction"`
	ServerURL      string                     `json:"server_url"`
	ThroughputMbps float64                    `json:"throughput_mbps"`
	LatencyMs      float64                    `json:"latency_ms"`
	JitterMs       float64                    `json:"jitter_ms"`
	BytesTotal     int64                      `json:"bytes_transferred"`
	DurationSec    float64                    `json:"duration_seconds"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

type diagnosisResult struct {
	ServerURL      string                     `json:"server_url"`
	LatencyMs      float64                    `json:"latency_ms"`
	JitterMs       float64                    `json:"jitter_ms"`
	DownloadMbps   float64                    `json:"download_mbps"`
	UploadMbps     float64                    `json:"upload_mbps"`
	Interpretation *diagnostic.Interpretation `json:"interpretation"`
}

func quickCheck(ctx context.Context, serverURL string) (*checkResult, error) {
	base := strings.TrimRight(serverURL, "/")
	client := &http.Client{Timeout: 5 * time.Second}

	// Health check
	if err := healthCheck(ctx, client, base); err != nil {
		return nil, err
	}

	// Latency
	avgLatency, jitter := measureLatency(ctx, client, base, 5)

	// Quick download burst
	downMbps := quickBurst(ctx, client, base, "download", 2)

	// Quick upload burst
	upMbps := quickBurst(ctx, client, base, "upload", 2)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
	})

	return &checkResult{
		Status:         "ok",
		ServerURL:      serverURL,
		LatencyMs:      avgLatency,
		DownloadMbps:   downMbps,
		UploadMbps:     upMbps,
		JitterMs:       jitter,
		Interpretation: interp,
	}, nil
}

func runSpeedTest(ctx context.Context, serverURL, direction string, durationSec int) (*speedTestResult, error) {
	base := strings.TrimRight(serverURL, "/")
	client := &http.Client{Timeout: time.Duration(durationSec+10) * time.Second}

	if err := healthCheck(ctx, client, base); err != nil {
		return nil, err
	}

	avgLatency, jitter := measureLatency(ctx, client, base, 5)

	start := time.Now()
	throughput := quickBurst(ctx, client, base, direction, durationSec)
	elapsed := time.Since(start)

	var downMbps, upMbps float64
	if direction == "download" {
		downMbps = throughput
	} else {
		upMbps = throughput
	}

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
	})

	return &speedTestResult{
		Direction:      direction,
		ServerURL:      serverURL,
		ThroughputMbps: throughput,
		LatencyMs:      avgLatency,
		JitterMs:       jitter,
		DurationSec:    elapsed.Seconds(),
		Interpretation: interp,
	}, nil
}

func runDiagnosis(ctx context.Context, serverURL string) (*diagnosisResult, error) {
	base := strings.TrimRight(serverURL, "/")
	client := &http.Client{Timeout: 15 * time.Second}

	if err := healthCheck(ctx, client, base); err != nil {
		return nil, err
	}

	avgLatency, jitter := measureLatency(ctx, client, base, 10)
	downMbps := quickBurst(ctx, client, base, "download", 5)
	upMbps := quickBurst(ctx, client, base, "upload", 5)

	interp := diagnostic.Interpret(diagnostic.Params{
		DownloadMbps: downMbps,
		UploadMbps:   upMbps,
		LatencyMs:    avgLatency,
		JitterMs:     jitter,
	})

	return &diagnosisResult{
		ServerURL:      serverURL,
		LatencyMs:      avgLatency,
		JitterMs:       jitter,
		DownloadMbps:   downMbps,
		UploadMbps:     upMbps,
		Interpretation: interp,
	}, nil
}

// --- Shared helpers ---

func healthCheck(ctx context.Context, client *http.Client, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

func measureLatency(ctx context.Context, client *http.Client, base string, samples int) (avgMs, jitterMs float64) {
	pingURL := base + "/api/v1/ping"
	var latencies []time.Duration

	for i := 0; i < samples; i++ {
		if ctx.Err() != nil {
			break
		}
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		latencies = append(latencies, time.Since(start))
	}

	if len(latencies) == 0 {
		return 0, 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	avgMs = float64(total) / float64(len(latencies)) / float64(time.Millisecond)

	if len(latencies) >= 2 {
		var jitterSum float64
		for i := 1; i < len(latencies); i++ {
			diff := latencies[i] - latencies[i-1]
			if diff < 0 {
				diff = -diff
			}
			jitterSum += float64(diff) / float64(time.Millisecond)
		}
		jitterMs = jitterSum / float64(len(latencies)-1)
	}

	return avgMs, jitterMs
}

func quickBurst(ctx context.Context, client *http.Client, base, direction string, durationSec int) float64 {
	if direction == "upload" {
		return quickUploadBurst(ctx, client, base, durationSec)
	}
	return quickDownloadBurst(ctx, client, base, durationSec)
}

func quickDownloadBurst(ctx context.Context, client *http.Client, base string, durationSec int) float64 {
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+2)*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s/api/v1/download?duration=%d&chunk=1048576", base, durationSec)
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Accept-Encoding", "identity")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return 0
	}

	buf := make([]byte, 64*1024)
	var totalBytes int64
	for {
		n, readErr := resp.Body.Read(buf)
		totalBytes += int64(n)
		if readErr != nil {
			break
		}
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
}

func quickUploadBurst(ctx context.Context, client *http.Client, base string, durationSec int) float64 {
	upCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+2)*time.Second)
	defer cancel()

	payload := make([]byte, 1024*1024)
	req, err := http.NewRequestWithContext(upCtx, http.MethodPost, base+"/api/v1/upload",
		strings.NewReader(string(payload)))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed <= 0 || resp.StatusCode != http.StatusOK {
		return 0
	}
	return float64(len(payload)*8) / elapsed.Seconds() / 1_000_000
}
