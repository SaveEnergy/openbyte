package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

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
