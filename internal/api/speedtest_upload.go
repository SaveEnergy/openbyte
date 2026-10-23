package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

type uploadResponse struct {
	Bytes          int64   `json:"bytes"`
	DurationMS     int64   `json:"duration_ms"`
	ThroughputMbps float64 `json:"throughput_mbps"`
}

func uploadReadDeadline(start time.Time, maxDurationSec int) time.Time {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	return start.Add(time.Duration(maxDurationSec) * time.Second)
}

func (h *SpeedTestHandler) readUploadBody(
	readCtx context.Context,
	body io.Reader,
	controller *http.ResponseController,
	deadline time.Time,
) (totalBytes int64, readFailed bool) {
	// The constructor and this method exclusively own the fixed-size pool.
	bufPtr := h.uploadBufPool.Get().(*[]byte)
	defer h.uploadBufPool.Put(bufPtr)
	buf := *bufPtr
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

func writeUploadResponse(w http.ResponseWriter, controller *http.ResponseController, totalBytes int64, startTime time.Time) {
	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	durationMs := elapsed.Milliseconds()
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	if controller != nil {
		_ = controller.SetWriteDeadline(time.Now().Add(2 * time.Second))
	}
	respondJSON(w, uploadResponse{
		Bytes:          totalBytes,
		DurationMS:     durationMs,
		ThroughputMbps: throughputMbps,
	}, http.StatusOK)
}
