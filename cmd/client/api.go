package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read error response: %w", readErr)
		}
		return nil, fmt.Errorf("server error: %d %s", resp.StatusCode, string(body))
	}

	var streamResp StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &streamResp, nil
}

func CancelStream(ctx context.Context, serverURL, streamID, apiKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	apiURL := serverURL + "/api/v1/stream/" + streamID + "/cancel"
	req, err := http.NewRequestWithContext(reqCtx, "POST", apiURL, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("cancel stream failed: status %d", resp.StatusCode)
	}
	return nil
}

func completeStream(ctx context.Context, config *Config, streamID string, metrics EngineMetrics) error {
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

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

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("complete stream marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(reqCtx, "POST", config.ServerURL+"/api/v1/stream/"+streamID+"/complete", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("complete stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		return fmt.Errorf("complete stream send: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("complete stream failed: status %d", resp.StatusCode)
	}
	return nil
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

	// Close connection on context cancellation to unblock ReadJSON immediately.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	receivedComplete := false
	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				if receivedComplete {
					return nil
				}
				return fmt.Errorf("websocket closed before completion message")
			}
			var netErr interface{ Timeout() bool }
			if errors.As(err, &netErr) && netErr.Timeout() {
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
			receivedComplete = true
			if msg.Results != nil {
				formatter.FormatComplete(msg.Results)
				return nil
			}
		case "error":
			return fmt.Errorf("test failed: %s", msg.Message)
		}
	}
}
