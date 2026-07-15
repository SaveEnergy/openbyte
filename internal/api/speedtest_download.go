package api

import (
	"errors"
	"net/http"
	"time"
)

func streamDownload(w http.ResponseWriter, r *http.Request, randomSource []byte, chunkSize int, duration time.Duration) {
	flusher, canFlush := w.(http.Flusher)
	streamDeadline := time.Now().Add(duration)
	writeDeadline := streamDeadline.Add(speedtestCloseGrace)
	controller := http.NewResponseController(w)
	writeCount := 0
	flushInterval := 8
	offset := 0
	var nextDeadlineRefresh time.Time

	for {
		now := time.Now()
		if !now.Before(streamDeadline) {
			break
		}
		if r.Context().Err() != nil {
			return
		}
		if !now.Before(nextDeadlineRefresh) {
			_ = refreshWriteDeadline(controller, writeDeadline)
			nextDeadlineRefresh = now.Add(speedtestDeadlineRefreshPeriod)
		}
		if writeChunkFromSource(w, randomSource, chunkSize, &offset) != nil {
			return
		}
		writeCount++
		if canFlush && writeCount%flushInterval == 0 {
			flusher.Flush()
		}
	}
	if canFlush {
		_ = refreshWriteDeadline(controller, writeDeadline)
		flusher.Flush()
	}
}

func writeChunkFromSource(w http.ResponseWriter, source []byte, chunkSize int, offset *int) error {
	if len(source) == 0 || chunkSize <= 0 || offset == nil {
		return errors.New("invalid chunk source")
	}

	remaining := chunkSize
	for remaining > 0 {
		start := *offset
		if start >= len(source) {
			start = 0
			*offset = 0
		}

		available := len(source) - start
		toWrite := min(remaining, available)
		if toWrite <= 0 {
			*offset = 0
			continue
		}

		if _, err := w.Write(source[start : start+toWrite]); err != nil {
			return err
		}

		remaining -= toWrite
		*offset = start + toWrite
		if *offset >= len(source) {
			*offset = 0
		}
	}
	return nil
}
