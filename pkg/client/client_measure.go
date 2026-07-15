package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/saveenergy/openbyte/internal/httptransfer"
)

const uploadMeasurementGrace = 10 * time.Second
const latencyMeasurementTimeout = 10 * time.Second

func (c *Client) measureLatency(ctx context.Context, samples int) (avgMs, jitterMs float64, ok bool) {
	latencyCtx, cancel := context.WithTimeout(ctx, latencyMeasurementTimeout)
	defer cancel()

	latencies := c.collectLatencySamples(latencyCtx, samples)
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
		latency, sampleOK := httptransfer.MeasurePing(c.httpClient, req, time.Now())
		if sampleOK {
			latencies = append(latencies, latency)
		}
	}
	return latencies
}

func latenciesSum(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, latency := range latencies {
		total += latency
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

func (c *Client) downloadBurst(ctx context.Context, durationSec int) (float64, bool) {
	mbps, _, ok := c.downloadMeasured(ctx, durationSec)
	return mbps, ok
}

func (c *Client) downloadMeasured(ctx context.Context, durationSec int) (mbps float64, totalBytes int64, ok bool) {
	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec+3)*time.Second)
	defer cancel()

	reqURL := fmt.Sprintf("%s%s?duration=%d&chunk=1048576", c.serverURL, pathDownload, durationSec)
	start := time.Now()
	buf := make([]byte, 64*1024)
	err := httptransfer.Download(dlCtx, c.httpClient, reqURL, buf, func(n int) {
		totalBytes += int64(n)
	})
	if err != nil {
		return 0, totalBytes, false
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
	upCtx, cancel := context.WithTimeout(ctx, time.Duration(durationSec)*time.Second+uploadMeasurementGrace)
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
		if err := httptransfer.Upload(ctx, c.httpClient, c.serverURL+pathUpload, payload); err != nil {
			return totalBytes, 0
		}
		totalBytes += int64(len(payload))
		iterations++
	}
	return totalBytes, time.Since(start)
}
