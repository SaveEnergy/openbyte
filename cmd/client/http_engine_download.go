package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/httptransfer"
)

func (e *HTTPTestEngine) runDownload(ctx context.Context) error {
	deadline := time.Now().Add(e.config.Duration)
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			if err := e.runDownloadStream(ctx, deadline, delay); err != nil {
				errCh <- err
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	close(errCh)
	return joinStreamErrors("download", errCh)
}

func (e *HTTPTestEngine) runDownloadStream(ctx context.Context, deadline time.Time, delay time.Duration) error {
	streamCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	if !waitForStreamDelay(streamCtx, delay) {
		return nil
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return nil
	}

	bufPtr, ok := e.bufferPool.Get().(*[]byte)
	if !ok || bufPtr == nil || len(*bufPtr) < clientBufferSize {
		bufPtr = newClientBuffer()
	}
	buf := *bufPtr
	defer e.bufferPool.Put(bufPtr)

	err := httptransfer.Download(
		streamCtx,
		e.client,
		e.buildDownloadURL(remaining),
		buf,
		func(n int) {
			e.addBytes(int64(n), e.elapsedSinceStart())
		},
	)
	if statusErr := asHTTPStatusError(err); statusErr != nil {
		return e.handleNonOKResponse(ctx, "download", statusErr)
	}
	if err != nil && streamCtx.Err() != nil {
		return nil
	}
	return err
}

func waitForStreamDelay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (e *HTTPTestEngine) buildDownloadURL(duration time.Duration) string {
	base := strings.TrimRight(e.config.ServerURL, "/")
	u, err := url.Parse(base + "/api/v1/download")
	if err != nil {
		return base + "/api/v1/download"
	}
	q := u.Query()
	q.Set("duration", fmt.Sprintf("%d", ceilDurationSeconds(duration)))
	q.Set("chunk", fmt.Sprintf("%d", e.config.ChunkSize))
	u.RawQuery = q.Encode()
	return u.String()
}

func ceilDurationSeconds(duration time.Duration) int {
	if duration <= 0 {
		return 1
	}
	seconds := int(duration / time.Second)
	if duration%time.Second != 0 {
		seconds++
	}
	return max(seconds, 1)
}
