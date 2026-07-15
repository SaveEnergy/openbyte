package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/httptransfer"
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

func asHTTPStatusError(err error) *httptransfer.StatusError {
	var statusErr *httptransfer.StatusError
	if errors.As(err, &statusErr) {
		return statusErr
	}
	return nil
}

func (e *HTTPTestEngine) handleNonOKResponse(
	ctx context.Context,
	direction string,
	statusErr *httptransfer.StatusError,
) error {
	if statusErr.Code == http.StatusTooManyRequests {
		wait := parseRetryAfter(statusErr.RetryAfter, time.Second)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
		}
	}
	return fmt.Errorf("%s failed: %s", direction, statusErr.Status)
}

func (e *HTTPTestEngine) addBytes(n int64, elapsed time.Duration) {
	if elapsed >= e.config.Duration {
		return
	}
	if elapsed < e.config.GraceTime {
		return
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
		if latency, ok := httptransfer.MeasurePing(client, req, start); ok {
			results = append(results, latency)
		}
	}
	return results, nil
}
