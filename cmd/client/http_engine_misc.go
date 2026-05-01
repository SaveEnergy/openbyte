package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

func joinStreamErrors(direction string, errCh <-chan error) error {
	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s streams failed: %w", direction, errors.Join(errs...))
}

func drainAndClose(resp *http.Response) {
	if resp == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func (e *HTTPTestEngine) handleNonOKResponse(ctx context.Context, direction string, resp *http.Response) error {
	if resp.StatusCode == http.StatusTooManyRequests {
		wait := parseRetryAfter(resp.Header.Get("Retry-After"), time.Second)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
		}
	}
	drainAndClose(resp)
	return fmt.Errorf("%s failed: %s", direction, resp.Status)
}

func (e *HTTPTestEngine) addBytes(n int64, elapsed time.Duration) {
	if elapsed >= e.config.Duration {
		return
	}
	if elapsed < e.config.GraceTime {
		atomic.AddInt64(&e.graceBytes, n)
		return
	}
	if atomic.CompareAndSwapInt32(&e.graceDone, 0, 1) {
		atomic.StoreInt64(&e.totalBytes, 0)
	}
	atomic.AddInt64(&e.totalBytes, n)
}

func parseRetryAfter(value string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	sec, err := strconv.Atoi(trimmed)
	if err != nil || sec < 1 {
		return fallback
	}
	d := time.Duration(sec) * time.Second
	if d > 2*time.Minute {
		return 2 * time.Minute
	}
	return d
}

func measureHTTPPing(ctx context.Context, serverURL string, samples int) ([]time.Duration, error) {
	if samples <= 0 {
		return nil, nil
	}
	pingURL := strings.TrimRight(serverURL, "/") + "/api/v1/ping"
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
	defer client.CloseIdleConnections()
	results := make([]time.Duration, 0, samples)

	for range samples {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			return results, err
		}
		resp, err := client.Do(req)
		if err != nil {
			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			results = append(results, time.Since(start))
		}
	}
	return results, nil
}
