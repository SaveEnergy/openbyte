package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c *Client) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+pathHealth, nil)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
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

func (c *Client) measureLatency(ctx context.Context, samples int) (avgMs, jitterMs float64, ok bool) {
	latencies := c.collectLatencySamples(ctx, samples)
	if len(latencies) < 2 {
		return 0, 0, false
	}
	avgMs = float64(latenciesSum(latencies)) / float64(len(latencies)) / float64(time.Millisecond)
	jitterMs = jitterFromLatencies(latencies)
	return avgMs, jitterMs, true
}

func (c *Client) collectLatencySamples(ctx context.Context, samples int) []time.Duration {
	pingURL := c.serverURL + pathPing
	var latencies []time.Duration
	for range samples {
		if ctx.Err() != nil {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			continue
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
		}
		latency, sampleOK := c.latencySample(req, time.Now())
		if sampleOK {
			latencies = append(latencies, latency)
		}
	}
	return latencies
}

func latenciesSum(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total
}

func jitterFromLatencies(latencies []time.Duration) float64 {
	var jitterSum float64
	for i := 1; i < len(latencies); i++ {
		diff := latencies[i] - latencies[i-1]
		if diff < 0 {
			diff = -diff
		}
		jitterSum += float64(diff) / float64(time.Millisecond)
	}
	return jitterSum / float64(len(latencies)-1)
}

func (c *Client) latencySample(req *http.Request, start time.Time) (time.Duration, bool) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}
	return time.Since(start), true
}

func (c *Client) downloadBurst(ctx context.Context, durationSec int) (float64, bool) {
	mbps, _, ok := c.downloadMeasured(ctx, durationSec)
	return mbps, ok
}

func (c *Client) downloadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64, ok bool) {
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s%s?duration=%d&chunk=1048576", c.serverURL, pathDownload, durationSec)
	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, false
	}
	req.Header.Set("Accept-Encoding", "identity")
	if c.apiKey != "" {
		req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return 0, 0, false
	}

	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		totalBytes += int64(n)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				return 0, totalBytes, false
			}
			break
		}
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0, totalBytes, false
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes, true
}

func (c *Client) uploadBurst(ctx context.Context, durationSec int) (float64, bool) {
	mbps, _, ok := c.uploadMeasured(ctx, durationSec)
	return mbps, ok
}

func (c *Client) uploadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64, ok bool) {
	upCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()
	totalBytes, elapsed := c.uploadLoop(upCtx, durationSec)
	if elapsed <= 0 || totalBytes == 0 {
		return 0, totalBytes, false
	}
	return float64(totalBytes*8) / elapsed.Seconds() / 1_000_000, totalBytes, true
}

func (c *Client) uploadLoop(ctx context.Context, durationSec int) (totalBytes int64, elapsed time.Duration) {
	payload := make([]byte, 1024*1024)
	targetDuration := time.Duration(durationSec) * time.Second
	start := time.Now()
	iterations := 0
	for {
		if ctx.Err() != nil {
			break
		}
		if iterations > 0 && time.Since(start) >= targetDuration {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+pathUpload,
			bytes.NewReader(payload))
		if err != nil {
			return totalBytes, 0
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		if c.apiKey != "" {
			req.Header.Set("Authorization", authBearerPrefix+c.apiKey)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return totalBytes, 0
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return totalBytes, 0
		}
		totalBytes += int64(len(payload))
		iterations++
	}
	return totalBytes, time.Since(start)
}
