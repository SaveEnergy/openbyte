// Package httptransfer contains the shared HTTP request/body loops used by
// openByte's CLI and Go SDK. Callers keep ownership of timing and concurrency.
package httptransfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// StatusError reports a completed non-OK response.
type StatusError struct {
	Code       int
	Status     string
	RetryAfter string
}

func (e *StatusError) Error() string { return e.Status }

// Download performs one speed-test download and reports each body read.
func Download(
	ctx context.Context,
	client *http.Client,
	requestURL string,
	buffer []byte,
	onBytes func(int),
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := doOK(client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 && onBytes != nil {
			onBytes(n)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

// Upload performs one speed-test upload and drains the response body.
func Upload(ctx context.Context, client *http.Client, requestURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := doOK(client, req)
	if err != nil {
		return err
	}
	drainAndClose(resp)
	return nil
}

// MeasurePing drains one prepared ping request and returns its round-trip time.
func MeasurePing(client *http.Client, req *http.Request, start time.Time) (time.Duration, bool) {
	resp, err := doOK(client, req)
	if err != nil {
		return 0, false
	}
	drainAndClose(resp)
	return time.Since(start), true
}

func doOK(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		drainAndClose(resp)
		return nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}
	statusErr := statusError(resp)
	drainAndClose(resp)
	return nil, statusErr
}

func statusError(resp *http.Response) *StatusError {
	return &StatusError{
		Code:       resp.StatusCode,
		Status:     resp.Status,
		RetryAfter: resp.Header.Get("Retry-After"),
	}
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
