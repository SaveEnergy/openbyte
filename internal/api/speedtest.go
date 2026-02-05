package api

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type SpeedTestHandler struct {
	activeDownloads  int64
	activeUploads    int64
	maxConcurrent    int64
	clientIPResolver *ClientIPResolver
	randomData       []byte
}

const speedtestRandomSize = 4 * 1024 * 1024

func NewSpeedTestHandler(maxConcurrent int) *SpeedTestHandler {
	handler := &SpeedTestHandler{
		maxConcurrent: int64(maxConcurrent),
		randomData:    make([]byte, speedtestRandomSize),
	}
	if _, err := rand.Read(handler.randomData); err != nil {
		handler.randomData = nil
	}
	return handler
}

func (h *SpeedTestHandler) SetClientIPResolver(resolver *ClientIPResolver) {
	h.clientIPResolver = resolver
}

func (h *SpeedTestHandler) Download(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt64(&h.activeDownloads) >= h.maxConcurrent {
		http.Error(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}
	atomic.AddInt64(&h.activeDownloads, 1)
	defer atomic.AddInt64(&h.activeDownloads, -1)

	durationStr := r.URL.Query().Get("duration")
	duration := 10 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err == nil && d > 0 && d <= 60 {
			duration = time.Duration(d) * time.Second
		}
	}

	chunkSize := 1048576
	if cs := r.URL.Query().Get("chunk"); cs != "" {
		if c, err := strconv.Atoi(cs); err == nil && c >= 65536 && c <= 4194304 {
			chunkSize = c
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	flusher, canFlush := w.(http.Flusher)

	var chunk []byte
	if len(h.randomData) < chunkSize {
		chunk = make([]byte, chunkSize)
		if _, err := rand.Read(chunk); err != nil {
			http.Error(w, "failed to generate random data", http.StatusInternalServerError)
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
			if len(h.randomData) >= chunkSize {
				if offset+chunkSize <= len(h.randomData) {
					_, err := w.Write(h.randomData[offset : offset+chunkSize])
					if err != nil {
						return
					}
					offset += chunkSize
					if offset == len(h.randomData) {
						offset = 0
					}
				} else {
					first := h.randomData[offset:]
					if _, err := w.Write(first); err != nil {
						return
					}
					remaining := chunkSize - len(first)
					if remaining > 0 {
						if _, err := w.Write(h.randomData[:remaining]); err != nil {
							return
						}
					}
					offset = remaining
				}
			} else {
				_, err := w.Write(chunk)
				if err != nil {
					return
				}
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
	if atomic.LoadInt64(&h.activeUploads) >= h.maxConcurrent {
		http.Error(w, "too many concurrent uploads", http.StatusServiceUnavailable)
		return
	}
	atomic.AddInt64(&h.activeUploads, 1)
	defer atomic.AddInt64(&h.activeUploads, -1)

	startTime := time.Now()

	totalBytes, err := io.Copy(io.Discard, r.Body)
	if err != nil {
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bytes":           totalBytes,
		"duration_ms":     elapsed.Milliseconds(),
		"throughput_mbps": throughputMbps,
	})
}

func (h *SpeedTestHandler) Ping(w http.ResponseWriter, r *http.Request) {
	clientIP := h.resolveClientIP(r)
	isIPv6 := strings.Contains(clientIP, ":")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pong":      true,
		"timestamp": time.Now().UnixMilli(),
		"client_ip": clientIP,
		"ipv6":      isIPv6,
	})
}

func (h *SpeedTestHandler) resolveClientIP(r *http.Request) string {
	if h.clientIPResolver == nil {
		return ipString(parseRemoteIP(r.RemoteAddr))
	}
	return h.clientIPResolver.FromRequest(r)
}
