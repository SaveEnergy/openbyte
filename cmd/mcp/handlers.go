package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/saveenergy/openbyte/pkg/client"
)

const (
	invalidServerURLFmt   = "Invalid server_url: %v"
	jsonEncodingFailedFmt = "JSON encoding failed: %v"
	connectivityFailedFmt = "Connectivity check failed: %v"
	speedTestInputFailed  = "Invalid speed_test input: %v"
	speedTestFailedFmt    = "Speed test failed: %v"
	diagnosisFailedFmt    = "Diagnosis failed: %v"
)

func handleConnectivityCheck(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const toolName = "connectivity_check"
	rawServerURL := req.GetString("server_url", defaultServerURL)
	serverURL, err := ValidateServerURL(rawServerURL)
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(invalidServerURLFmt, err), map[string]any{
			"server_url":  rawServerURL,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.Check(ctx)
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(connectivityFailedFmt, err), map[string]any{
			"server_url":  serverURL,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(jsonEncodingFailedFmt, err), map[string]any{
			"server_url": serverURL,
		}), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleSpeedTest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const toolName = "speed_test"
	rawServerURL := req.GetString("server_url", defaultServerURL)
	serverURL, err := ValidateServerURL(rawServerURL)
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(invalidServerURLFmt, err), map[string]any{
			"server_url":  rawServerURL,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}
	direction := req.GetString("direction", "download")
	duration := req.GetInt("duration", 10)

	if err := ValidateSpeedTestInput(direction, duration); err != nil {
		return toolResultError(toolName, fmt.Errorf(speedTestInputFailed, err), map[string]any{
			"server_url":  serverURL,
			"direction":   direction,
			"duration":    duration,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.SpeedTest(ctx, client.SpeedTestOptions{
		Direction: direction,
		Duration:  duration,
	})
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(speedTestFailedFmt, err), map[string]any{
			"server_url":  serverURL,
			"direction":   direction,
			"duration":    duration,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(jsonEncodingFailedFmt, err), map[string]any{
			"server_url": serverURL,
			"direction":  direction,
			"duration":   duration,
		}), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleDiagnose(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const toolName = "diagnose"
	rawServerURL := req.GetString("server_url", defaultServerURL)
	serverURL, err := ValidateServerURL(rawServerURL)
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(invalidServerURLFmt, err), map[string]any{
			"server_url":  rawServerURL,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	c := clientFromRequest(serverURL, req)
	result, err := c.Diagnose(ctx)
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(diagnosisFailedFmt, err), map[string]any{
			"server_url":  serverURL,
			"api_key_set": strings.TrimSpace(req.GetString("api_key", "")) != "",
		}), nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return toolResultError(toolName, fmt.Errorf(jsonEncodingFailedFmt, err), map[string]any{
			"server_url": serverURL,
		}), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func toolResultError(tool string, err error, input map[string]any) *mcp.CallToolResult {
	inputJSON, marshalErr := json.Marshal(input)
	if marshalErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool %s failed: %v", tool, err))
	}
	return mcp.NewToolResultError(fmt.Sprintf("Tool %s failed: %v (input=%s)", tool, err, string(inputJSON)))
}

func clientFromRequest(serverURL string, req mcp.CallToolRequest) *client.Client {
	apiKey := strings.TrimSpace(req.GetString("api_key", ""))
	if apiKey == "" {
		return client.New(serverURL)
	}
	return client.New(serverURL, client.WithAPIKey(apiKey))
}

// ValidateServerURL parses and validates an HTTP(S) base URL for MCP tools.
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

// ValidateSpeedTestInput validates direction and duration for speed_test.
func ValidateSpeedTestInput(direction string, duration int) error {
	if direction != "download" && direction != "upload" {
		return fmt.Errorf("direction must be download or upload")
	}
	if duration < 1 || duration > 300 {
		return fmt.Errorf("duration must be 1-300")
	}
	return nil
}
