// Package mcp implements the `openbyte mcp` subcommand — an MCP (Model Context
// Protocol) server over stdio transport. Agents can spawn this process and call
// connectivity tools directly.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	registerTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "openbyte mcp: error: %v\n", err)
		return 1
	}
	return 0
}

func registerTools(s *server.MCPServer) {
	s.AddTool(connectivityCheckTool(), handleConnectivityCheck)
	s.AddTool(speedTestTool(), handleSpeedTest)
	s.AddTool(diagnoseTool(), handleDiagnose)
}

// ToolDefinitions exposes MCP tool schemas for contract tests.
func ToolDefinitions() []mcp.Tool {
	return []mcp.Tool{
		connectivityCheckTool(),
		speedTestTool(),
		diagnoseTool(),
	}
}

func connectivityCheckTool() mcp.Tool {
	return mcp.NewTool("connectivity_check",
		mcp.WithDescription("Quick connectivity check (~3-5 seconds). Returns latency, rough download/upload speed, grade (A-F), and diagnostic interpretation. Use this for fast 'is the network OK?' checks."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
		mcp.WithString("api_key",
			mcp.Description("Optional bearer API key for authenticated endpoints"),
		),
	)
}

func speedTestTool() mcp.Tool {
	return mcp.NewTool("speed_test",
		mcp.WithDescription("Full speed test with configurable duration. Returns detailed throughput, latency, jitter, and diagnostic interpretation. Use for accurate measurements."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
		mcp.WithString("direction",
			mcp.Description("Test direction: download or upload (default: download)"),
		),
		mcp.WithNumber("duration",
			mcp.Description("Test duration in seconds, 1-300 (default: 10)"),
		),
		mcp.WithString("api_key",
			mcp.Description("Optional bearer API key for authenticated endpoints"),
		),
	)
}

func diagnoseTool() mcp.Tool {
	return mcp.NewTool("diagnose",
		mcp.WithDescription("Comprehensive network diagnosis: measures latency, download speed, upload speed, and returns bufferbloat grade, suitability assessment, and concerns. Takes ~15-20 seconds."),
		mcp.WithString("server_url",
			mcp.Description("Speed test server URL (default: http://localhost:8080)"),
		),
		mcp.WithString("api_key",
			mcp.Description("Optional bearer API key for authenticated endpoints"),
		),
	)
}

// --- Tool Handlers ---

func handleConnectivityCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverURL, err := ValidateServerURL(req.GetString("server_url", "http://localhost:8080"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid server_url: %v", err)), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.Check(ctx)
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
	serverURL, err := ValidateServerURL(req.GetString("server_url", "http://localhost:8080"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid server_url: %v", err)), nil
	}
	direction := req.GetString("direction", "download")
	duration := req.GetInt("duration", 10)

	if err := ValidateSpeedTestInput(direction, duration); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid speed_test input: %v", err)), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.SpeedTest(ctx, client.SpeedTestOptions{
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
	serverURL, err := ValidateServerURL(req.GetString("server_url", "http://localhost:8080"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid server_url: %v", err)), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.Diagnose(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Diagnosis failed: %v", err)), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON encoding failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func clientFromRequest(serverURL string, req mcp.CallToolRequest) *client.Client {
	apiKey := strings.TrimSpace(req.GetString("api_key", ""))
	if apiKey == "" {
		return client.New(serverURL)
	}
	return client.New(serverURL, client.WithAPIKey(apiKey))
}

func ValidateServerURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("host is required")
	}
	if u.RawQuery != "" {
		return "", fmt.Errorf("query is not allowed")
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("fragment is not allowed")
	}
	if port := u.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("port must be in range 1-65535")
		}
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func ValidateSpeedTestInput(direction string, duration int) error {
	if direction != "download" && direction != "upload" {
		return fmt.Errorf("direction must be download or upload")
	}
	if duration < 1 || duration > 300 {
		return fmt.Errorf("duration must be 1-300")
	}
	return nil
}
