package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

func uploadReadDeadline(start time.Time, maxDurationSec int) time.Time {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	return start.Add(time.Duration(maxDurationSec) * time.Second)
}

func readUploadBody(
	readCtx context.Context,
	body io.Reader,
	controller *http.ResponseController,
	deadline time.Time,
	pool *sync.Pool,
) (totalBytes int64, readFailed bool) {
	bufPtr := getUploadBuf(pool)
	buf := *bufPtr
	if pool != nil {
		defer pool.Put(bufPtr)
	}
	for {
		select {
		case <-readCtx.Done():
			return totalBytes, false
		default:
		}
		if time.Now().After(deadline) {
			return totalBytes, false
		}
		_ = refreshReadDeadline(controller, deadline)
		n, err := body.Read(buf)
		totalBytes += int64(n)
		if err != nil {
			return totalBytes, !errors.Is(err, io.EOF)
		}
	}
}

func getUploadBuf(pool *sync.Pool) *[]byte {
	if pool == nil {
		return newUploadBuffer()
	}
	pooled := pool.Get()
	if pooled == nil {
		return newUploadBuffer()
	}
	if cast, ok := pooled.(*[]byte); ok && cast != nil && cap(*cast) >= uploadReadBufferSize {
		buf := (*cast)[:uploadReadBufferSize]
		*cast = buf
		return cast
	}
	return newUploadBuffer()
}

func writeUploadResponse(w http.ResponseWriter, controller *http.ResponseController, totalBytes int64, startTime time.Time) {
	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	w.Header().Set(headerContentType, contentTypeJSON)
	_ = controller.SetWriteDeadline(time.Now().Add(2 * time.Second))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"bytes":           totalBytes,
		"duration_ms":     elapsed.Milliseconds(),
		"throughput_mbps": throughputMbps,
	}); err != nil {
		logging.Warn("speedtest: encode upload response", logging.Field{Key: "error", Value: err})
	}
}
