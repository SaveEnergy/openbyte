package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
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
	var nextDeadlineRefresh time.Time
	for {
		select {
		case <-readCtx.Done():
			return totalBytes, false
		default:
		}
		now := time.Now()
		if now.After(deadline) {
			return totalBytes, false
		}
		if controller != nil && !now.Before(nextDeadlineRefresh) {
			_ = refreshReadDeadline(controller, deadline)
			nextDeadlineRefresh = now.Add(speedtestDeadlineRefreshPeriod)
		}
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
	durationMs := elapsed.Milliseconds()
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	w.Header().Set(headerContentType, contentTypeJSON)
	if controller != nil {
		_ = controller.SetWriteDeadline(time.Now().Add(2 * time.Second))
	}
	w.WriteHeader(http.StatusOK)
	var buf [128]byte
	payload := appendUploadResponseJSON(buf[:0], totalBytes, durationMs, throughputMbps)
	if _, err := w.Write(payload); err != nil {
		logging.Warn("speedtest: write upload response", logging.Field{Key: "error", Value: err})
	}
}

func appendUploadResponseJSON(dst []byte, totalBytes, durationMs int64, throughputMbps float64) []byte {
	dst = append(dst, `{"bytes":`...)
	dst = strconv.AppendInt(dst, totalBytes, 10)
	dst = append(dst, `,"duration_ms":`...)
	dst = strconv.AppendInt(dst, durationMs, 10)
	dst = append(dst, `,"throughput_mbps":`...)
	dst = strconv.AppendFloat(dst, throughputMbps, 'f', -1, 64)
	dst = append(dst, '}')
	return dst
}
