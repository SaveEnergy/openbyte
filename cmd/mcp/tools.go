package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	defaultServerURL    = "http://localhost:8080"
	serverURLDescPrefix = "Speed test server URL (default: "
	optionalAPIKeyDesc  = "Optional bearer API key for authenticated endpoints"
)

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
			mcp.Description(serverURLDescPrefix+defaultServerURL+")"),
		),
		mcp.WithString("api_key",
			mcp.Description(optionalAPIKeyDesc),
		),
	)
}

func speedTestTool() mcp.Tool {
	return mcp.NewTool("speed_test",
		mcp.WithDescription("Full speed test with configurable duration. Returns detailed throughput, latency, jitter, and diagnostic interpretation. Use for accurate measurements."),
		mcp.WithString("server_url",
			mcp.Description(serverURLDescPrefix+defaultServerURL+")"),
		),
		mcp.WithString("direction",
			mcp.Description("Test direction: download or upload (default: download)"),
		),
		mcp.WithNumber("duration",
			mcp.Description("Test duration in seconds, 1-300 (default: 10)"),
		),
		mcp.WithString("api_key",
			mcp.Description(optionalAPIKeyDesc),
		),
	)
}

func diagnoseTool() mcp.Tool {
	return mcp.NewTool("diagnose",
		mcp.WithDescription("Comprehensive network diagnosis: measures latency, download speed, upload speed, and returns bufferbloat grade, suitability assessment, and concerns. Takes ~15-20 seconds."),
		mcp.WithString("server_url",
			mcp.Description(serverURLDescPrefix+defaultServerURL+")"),
		),
		mcp.WithString("api_key",
			mcp.Description(optionalAPIKeyDesc),
		),
	)
}
