package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	apiPathStreamStart = "/api/v1/stream/start"
	apiPathStreamBase  = "/api/v1/stream/"
	contentTypeJSON    = "application/json"
	pathCancel         = "/cancel"
	pathComplete       = "/complete"
	statusCompleted    = "completed"
	maxErrorBodyBytes  = 8 * 1024
)

func startStream(ctx context.Context, config *Config) (*StreamResponse, error) {
	reqBody := StartStreamRequest{
		Protocol:   config.Protocol,
		Direction:  config.Direction,
		Duration:   config.Duration,
		Streams:    config.Streams,
		PacketSize: config.PacketSize,
		Mode:       modeClient,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := config.ServerURL + apiPathStreamStart
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := newHTTPClient(timeout)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, readErr := readBoundedErrorBody(resp.Body, maxErrorBodyBytes)
		if readErr != nil {
			return nil, fmt.Errorf("read error response: %w", readErr)
		}
		if body == "" {
			body = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("server error: %d %s", resp.StatusCode, body)
	}

	var streamResp StreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&streamResp); err != nil {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &streamResp, nil
}

func CancelStream(ctx context.Context, serverURL, streamID string) error {
	parent := ctx
	if parent == nil {
		parent = context.Background()
	}
	parent = context.WithoutCancel(parent)
	reqCtx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	apiURL := serverURL + apiPathStreamBase + streamID + pathCancel
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, nil)
	if err != nil {
		return err
	}

	client := newHTTPClient(5 * time.Second)
	defer client.CloseIdleConnections()
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

	reqBody := map[string]any{
		"status": statusCompleted,
		"metrics": map[string]any{
			"throughput_mbps":     metrics.ThroughputMbps,
			"throughput_avg_mbps": metrics.ThroughputMbps,
			"bytes_transferred":   metrics.BytesTransferred,
			"jitter_ms":           metrics.JitterMs,
			"latency_ms": map[string]any{
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
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, config.ServerURL+apiPathStreamBase+streamID+pathComplete, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("complete stream request: %w", err)
	}
	req.Header.Set("Content-Type", contentTypeJSON)

	client := newHTTPClient(5 * time.Second)
	defer client.CloseIdleConnections()
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

func readBoundedErrorBody(body io.Reader, limit int64) (string, error) {
	if body == nil {
		return "", nil
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(io.Discard, body); err != nil {
		return "", err
	}
	truncated := int64(len(data)) > limit
	if truncated {
		data = data[:limit]
	}
	message := strings.TrimSpace(string(data))
	if truncated && message != "" {
		message += "... (truncated)"
	}
	return message, nil
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}
