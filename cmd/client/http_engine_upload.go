package client

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/httptransfer"
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
		err := httptransfer.Upload(ctx, e.client, reqURL, e.uploadPayload)
		if err != nil {
			if statusErr := asHTTPStatusError(err); statusErr != nil {
				return e.handleNonOKResponse(ctx, "upload", statusErr)
			}
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		e.addBytes(int64(len(e.uploadPayload)), e.elapsedSinceStart())
		now = time.Now()
	}
	return nil
}

func (e *HTTPTestEngine) buildUploadURL() string {
	return strings.TrimRight(e.config.ServerURL, "/") + "/api/v1/upload"
}
