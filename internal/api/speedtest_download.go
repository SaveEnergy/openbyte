package api

import (
	"crypto/rand"
	"errors"
	"net/http"
	"time"
)

// resolveRandomSource returns data source and release func. Call release when done.
func (h *SpeedTestHandler) resolveRandomSource() ([]byte, func(), error) {
	if len(h.randomData) != 0 {
		return h.randomData, func() {
			// Intentionally empty: shared randomData has no per-request cleanup.
		}, nil
	}
	pooled, _ := h.fallbackRandomPool.Get().([]byte)
	const fallbackSize = 64 * 1024
	if len(pooled) < fallbackSize {
		pooled = make([]byte, fallbackSize)
	}
	randomSource := pooled[:fallbackSize]
	if _, err := rand.Read(randomSource); err != nil {
		if cap(pooled) >= fallbackSize {
			h.fallbackRandomPool.Put(pooled)
		}
		return nil, nil, err
	}
	return randomSource, func() {
		if cap(pooled) >= fallbackSize {
			h.fallbackRandomPool.Put(pooled)
		}
	}, nil
}

func streamDownload(w http.ResponseWriter, r *http.Request, randomSource []byte, chunkSize int, duration time.Duration) {
	flusher, canFlush := w.(http.Flusher)
	streamDeadline := time.Now().Add(duration)
	writeDeadline := streamDeadline.Add(speedtestCloseGrace)
	controller := http.NewResponseController(w)
	writeCount := 0
	flushInterval := 8
	offset := 0

	for time.Now().Before(streamDeadline) {
		if r.Context().Err() != nil {
			return
		}
		_ = refreshWriteDeadline(controller, writeDeadline)
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
