package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func startStream(ctx context.Context, config *Config) (*StreamResponse, error) {
	reqBody := StartStreamRequest{
		Protocol:   config.Protocol,
		Direction:  config.Direction,
		Duration:   config.Duration,
		Streams:    config.Streams,
		PacketSize: config.PacketSize,
		Mode:       "client",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := config.ServerURL + "/api/v1/stream/start"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	client := &http.Client{Timeout: time.Duration(config.Timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %d %s", resp.StatusCode, string(body))
	}

	var streamResp StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &streamResp, nil
}

func cancelStream(serverURL, streamID, apiKey string) {
	apiURL := serverURL + "/api/v1/stream/" + streamID + "/cancel"
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	client.Do(req)
}

func completeStream(config *Config, streamID string, metrics EngineMetrics) {
	reqBody := map[string]interface{}{
		"status": "completed",
		"metrics": map[string]interface{}{
			"throughput_mbps":     metrics.ThroughputMbps,
			"throughput_avg_mbps": metrics.ThroughputMbps,
			"bytes_transferred":   metrics.BytesTransferred,
			"jitter_ms":           metrics.JitterMs,
			"latency_ms": map[string]interface{}{
				"min_ms": metrics.Latency.MinMs,
				"max_ms": metrics.Latency.MaxMs,
				"avg_ms": metrics.Latency.AvgMs,
				"p50_ms": metrics.Latency.P50Ms,
				"p95_ms": metrics.Latency.P95Ms,
				"p99_ms": metrics.Latency.P99Ms,
				"count":  metrics.Latency.Count,
			},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", config.ServerURL+"/api/v1/stream/"+streamID+"/complete", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	client.Do(req)
}

func streamMetrics(ctx context.Context, wsURL string, formatter OutputFormatter, config *Config) error {
	if strings.HasPrefix(wsURL, "/") {
		serverURL, err := url.Parse(config.ServerURL)
		if err != nil {
			return fmt.Errorf("parse server URL: %w", err)
		}
		wsScheme := "ws"
		if serverURL.Scheme == "https" {
			wsScheme = "wss"
		}
		wsURL = fmt.Sprintf("%s://%s%s", wsScheme, serverURL.Host, wsURL)
	} else {
		u, err := url.Parse(wsURL)
		if err != nil {
			return fmt.Errorf("parse websocket URL: %w", err)
		}
		if u.Scheme == "http" {
			u.Scheme = "ws"
		} else if u.Scheme == "https" {
			u.Scheme = "wss"
		}
		wsURL = u.String()
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer conn.Close()

	readTimeout := 30 * time.Second
	if config.Timeout > 0 {
		readTimeout = time.Duration(config.Timeout) * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn.SetReadDeadline(time.Now().Add(readTimeout))

			var msg WebSocketMessage
			if err := conn.ReadJSON(&msg); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return nil
				}
				if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					return fmt.Errorf("websocket read timeout: %w", err)
				}
				return fmt.Errorf("read message: %w", err)
			}

			switch msg.Type {
			case "progress":
				formatter.FormatProgress(msg.Progress, msg.ElapsedSeconds, msg.RemainingSeconds)
			case "metrics":
				if msg.Metrics != nil {
					formatter.FormatMetrics(msg.Metrics)
				}
			case "complete":
				if msg.Results != nil {
					formatter.FormatComplete(msg.Results)
					return nil
				}
			case "error":
				return fmt.Errorf("test failed: %s", msg.Message)
			}
		}
	}
}
