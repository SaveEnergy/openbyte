package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

type SpeedTestHandler struct {
	activeDownloads  int64
	activeUploads    int64
	maxConcurrent    int64
	maxDurationSec   int
	clientIPResolver *ClientIPResolver
	randomData       []byte
}

const speedtestRandomSize = 4 * 1024 * 1024

func NewSpeedTestHandler(maxConcurrent int, maxDurationSec int) *SpeedTestHandler {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	handler := &SpeedTestHandler{
		maxConcurrent:  int64(maxConcurrent),
		maxDurationSec: maxDurationSec,
		randomData:     make([]byte, speedtestRandomSize),
	}
	if _, err := rand.Read(handler.randomData); err != nil {
		logging.Warn("speedtest: random data init failed, using per-request random",
			logging.Field{Key: "error", Value: err})
		handler.randomData = nil
	}
	return handler
}

func (h *SpeedTestHandler) SetClientIPResolver(resolver *ClientIPResolver) {
	h.clientIPResolver = resolver
}

func respondSpeedtestError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logging.Warn("speedtest: encode error response", logging.Field{Key: "error", Value: err})
	}
}

func (h *SpeedTestHandler) Download(w http.ResponseWriter, r *http.Request) {
	if v := atomic.AddInt64(&h.activeDownloads, 1); v > h.maxConcurrent {
		atomic.AddInt64(&h.activeDownloads, -1)
		drainRequestBody(r)
		respondSpeedtestError(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}
	defer atomic.AddInt64(&h.activeDownloads, -1)

	duration, chunkSize, parseErr := parseDownloadParams(r, h.maxDurationSec)
	if parseErr != nil {
		drainRequestBody(r)
		respondSpeedtestError(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")

	randomSource, err := h.resolveRandomSource()
	if err != nil {
		drainRequestBody(r)
		respondSpeedtestError(w, "failed to generate random data", http.StatusInternalServerError)
		return
	}

	streamDownload(w, r, randomSource, chunkSize, duration)
}

func (h *SpeedTestHandler) Upload(w http.ResponseWriter, r *http.Request) {
	defer drainRequestBody(r)

	if v := atomic.AddInt64(&h.activeUploads, 1); v > h.maxConcurrent {
		atomic.AddInt64(&h.activeUploads, -1)
		respondSpeedtestError(w, "too many concurrent uploads", http.StatusServiceUnavailable)
		return
	}
	defer atomic.AddInt64(&h.activeUploads, -1)

	startTime := time.Now()
	deadline := uploadReadDeadline(startTime, h.maxDurationSec)
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(deadline)
	readCtx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()
	var closeBodyOnce sync.Once
	go func() {
		<-readCtx.Done()
		if errors.Is(readCtx.Err(), context.DeadlineExceeded) {
			closeBodyOnce.Do(func() {
				_ = r.Body.Close()
			})
		}
	}()

	buf := make([]byte, 256*1024)
	var totalBytes int64
	var readFailed bool
	now := time.Now()
	readIterations := 0
	for {
		select {
		case <-readCtx.Done():
			goto done
		default:
		}
		n, err := r.Body.Read(buf)
		totalBytes += int64(n)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				readFailed = true
			}
			break
		}
		readIterations++
		if readIterations%32 == 0 {
			now = time.Now()
		}
		if now.After(deadline) {
			break
		}
	}
done:
	if readFailed {
		respondSpeedtestError(w, "upload failed", http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	w.Header().Set("Content-Type", "application/json")
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

func uploadReadDeadline(start time.Time, maxDurationSec int) time.Time {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	return start.Add(time.Duration(maxDurationSec) * time.Second)
}

func parseDownloadParams(r *http.Request, maxDurationSec int) (time.Duration, int, error) {
	duration := 10 * time.Second
	if durationStr := r.URL.Query().Get("duration"); durationStr != "" {
		d, err := strconv.Atoi(durationStr)
		if err != nil || d < 1 || d > maxDurationSec {
			return 0, 0, errors.New("duration must be 1-" + strconv.Itoa(maxDurationSec))
		}
		duration = time.Duration(d) * time.Second
	}

	chunkSize := 1048576
	if cs := r.URL.Query().Get("chunk"); cs != "" {
		c, err := strconv.Atoi(cs)
		if err != nil || c < 65536 || c > 4194304 {
			return 0, 0, errors.New("chunk must be 65536-4194304")
		}
		chunkSize = c
	}
	return duration, chunkSize, nil
}

func (h *SpeedTestHandler) resolveRandomSource() ([]byte, error) {
	randomSource := h.randomData
	if len(randomSource) != 0 {
		return randomSource, nil
	}
	// Keep fallback allocation bounded; stream logic handles chunk expansion.
	randomSource = make([]byte, 64*1024)
	if _, err := rand.Read(randomSource); err != nil {
		return nil, err
	}
	return randomSource, nil
}

func streamDownload(w http.ResponseWriter, r *http.Request, randomSource []byte, chunkSize int, duration time.Duration) {
	flusher, canFlush := w.(http.Flusher)
	deadline := time.Now().Add(duration)
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(deadline.Add(5 * time.Second))
	writeCount := 0
	flushInterval := 8
	offset := 0
	now := time.Now()
	deadlineTicker := time.NewTicker(100 * time.Millisecond)
	defer deadlineTicker.Stop()

	for now.Before(deadline) {
		select {
		case <-r.Context().Done():
			return
		case <-deadlineTicker.C:
			now = time.Now()
		default:
			if err := writeChunkFromSource(w, randomSource, chunkSize, &offset); err != nil {
				return
			}
			writeCount++
			if writeCount%64 == 0 {
				now = time.Now()
			}
			if canFlush && writeCount%flushInterval == 0 {
				flusher.Flush()
			}
		}
	}
	if canFlush {
		flusher.Flush()
	}
}

func (h *SpeedTestHandler) Ping(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	clientIP := h.resolveClientIP(r)
	isIPv6 := strings.Contains(clientIP, ":")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"pong":      true,
		"timestamp": time.Now().UnixMilli(),
		"client_ip": clientIP,
		"ipv6":      isIPv6,
	}); err != nil {
		logging.Warn("speedtest: encode ping response", logging.Field{Key: "error", Value: err})
	}
}

func (h *SpeedTestHandler) resolveClientIP(r *http.Request) string {
	if h.clientIPResolver == nil {
		return ipString(parseRemoteIP(r.RemoteAddr))
	}
	return h.clientIPResolver.FromRequest(r)
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
