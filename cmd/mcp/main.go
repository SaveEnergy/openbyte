// Package mcp implements the `openbyte mcp` subcommand — an MCP (Model Context
// Protocol) server over stdio transport. Agents can spawn this process and call
// connectivity tools directly.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/saveenergy/openbyte/pkg/client"
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

	c := client.New(serverURL)
	result, err := c.Check(checkCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Connectivity check failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON encoding failed: %v", err)), nil
	}
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

	c := client.New(serverURL)
	result, err := c.SpeedTest(testCtx, client.SpeedTestOptions{
		Direction: direction,
		Duration:  duration,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Speed test failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON encoding failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleDiagnose(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverURL := req.GetString("server_url", "http://localhost:8080")

	diagCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	c := client.New(serverURL)
	result, err := c.Diagnose(diagCtx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diagnosis failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON encoding failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
