package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

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
