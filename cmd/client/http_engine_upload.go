package client

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (e *HTTPTestEngine) runUpload(ctx context.Context) error {
	reqURL := e.buildUploadURL()
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)
	deadline := time.Now().Add(e.config.Duration)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			if err := e.runUploadStream(ctx, reqURL, deadline, delay); err != nil {
				errCh <- err
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	close(errCh)
	return joinStreamErrors("upload", errCh)
}

func (e *HTTPTestEngine) runUploadStream(ctx context.Context, reqURL string, deadline time.Time, delay time.Duration) error {
	time.Sleep(delay)
	now := time.Now()
	for now.Before(deadline) && ctx.Err() == nil {
		reader := bytes.NewReader(e.uploadPayload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := e.client.Do(req)
		if err != nil {
			drainAndClose(resp)
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return e.handleNonOKResponse(ctx, "upload", resp)
		}
		drainAndClose(resp)

		atomic.AddInt64(&e.bytesSent, int64(len(e.uploadPayload)))
		e.addBytes(int64(len(e.uploadPayload)), e.elapsedSinceStart())
		now = time.Now()
	}
	return nil
}

func (e *HTTPTestEngine) buildUploadURL() string {
	return strings.TrimRight(e.config.ServerURL, "/") + "/api/v1/upload"
}
