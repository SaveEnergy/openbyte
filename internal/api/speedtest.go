package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
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

	durationStr := r.URL.Query().Get("duration")
	duration := 10 * time.Second
	if durationStr != "" {
		d, err := strconv.Atoi(durationStr)
		if err != nil || d < 1 || d > h.maxDurationSec {
			drainRequestBody(r)
			respondSpeedtestError(w, "duration must be 1-"+strconv.Itoa(h.maxDurationSec), http.StatusBadRequest)
			return
		}
		duration = time.Duration(d) * time.Second
	}

	chunkSize := 1048576
	if cs := r.URL.Query().Get("chunk"); cs != "" {
		c, err := strconv.Atoi(cs)
		if err != nil || c < 65536 || c > 4194304 {
			drainRequestBody(r)
			respondSpeedtestError(w, "chunk must be 65536-4194304", http.StatusBadRequest)
			return
		}
		chunkSize = c
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")

	flusher, canFlush := w.(http.Flusher)
	randomSource := h.randomData
	if len(randomSource) == 0 {
		// Keep fallback allocation bounded; stream logic handles chunk expansion.
		randomSource = make([]byte, 64*1024)
		if _, err := rand.Read(randomSource); err != nil {
			drainRequestBody(r)
			respondSpeedtestError(w, "failed to generate random data", http.StatusInternalServerError)
			return
		}
	}

	deadline := time.Now().Add(duration)
	writeCount := 0
	flushInterval := 8
	offset := 0

	for time.Now().Before(deadline) {
		select {
		case <-r.Context().Done():
			return
		default:
			if err := writeChunkFromSource(w, randomSource, chunkSize, &offset); err != nil {
				return
			}
			writeCount++
			if canFlush && writeCount%flushInterval == 0 {
				flusher.Flush()
			}
		}
	}
	if canFlush {
		flusher.Flush()
	}
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

	buf := make([]byte, 256*1024)
	var totalBytes int64
	var readFailed bool
	for time.Now().Before(deadline) {
		select {
		case <-r.Context().Done():
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
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
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

func (h *SpeedTestHandler) Ping(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	clientIP := h.resolveClientIP(r)
	isIPv6 := strings.Contains(clientIP, ":")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
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
		toWrite := remaining
		if toWrite > available {
			toWrite = available
		}
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
