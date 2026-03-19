package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (e *HTTPTestEngine) runDownload(ctx context.Context) error {
	reqURL := e.buildDownloadURL()
	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	for i := 0; i < e.config.Streams; i++ {
		wg.Add(1)
		go func(delay time.Duration) {
			defer wg.Done()
			if err := e.runDownloadStream(ctx, reqURL, delay); err != nil {
				errCh <- err
			}
		}(time.Duration(i) * e.config.StreamDelay)
	}

	wg.Wait()
	close(errCh)
	return joinStreamErrors("download", errCh)
}

func (e *HTTPTestEngine) runDownloadStream(ctx context.Context, reqURL string, delay time.Duration) error {
	time.Sleep(delay)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "identity")
	if e.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.config.APIKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		drainAndClose(resp)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return e.handleNonOKResponse(ctx, "download", resp)
	}

	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
	defer e.bufferPool.Put(buf)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			atomic.AddInt64(&e.bytesReceived, int64(n))
			e.addBytes(int64(n), e.elapsedSinceStart())
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || ctx.Err() != nil {
				return nil
			}
			return readErr
		}
	}
}

func (e *HTTPTestEngine) buildDownloadURL() string {
	base := strings.TrimRight(e.config.ServerURL, "/")
	u, err := url.Parse(base + "/api/v1/download")
	if err != nil {
		return base + "/api/v1/download"
	}
	q := u.Query()
	q.Set("duration", fmt.Sprintf("%d", int(e.config.Duration.Seconds())))
	q.Set("chunk", fmt.Sprintf("%d", e.config.ChunkSize))
	u.RawQuery = q.Encode()
	return u.String()
}
