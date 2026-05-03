package client

import (
	"context"
	"io"
	"net/http"
	"time"
)

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
