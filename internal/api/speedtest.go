package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

type SpeedTestHandler struct {
	activeDownloads    int64
	activeUploads      int64
	maxConcurrent      int64
	maxConcurrentPerIP int
	maxDurationSec     int
	clientIPResolver   *ClientIPResolver
	randomData         []byte
	uploadBufPool      sync.Pool
	fallbackRandomPool sync.Pool
	ipMu               sync.Mutex
	activeByIP         map[string]*speedtestIPCounts
}

type speedtestIPCounts struct {
	downloads int
	uploads   int
}

const (
	speedtestRandomSize    = 4 * 1024 * 1024
	uploadReadBufferSize   = 256 * 1024
	headerContentType      = "Content-Type"
	contentTypeJSON        = "application/json"
	contentTypeOctetStream = "application/octet-stream"
)

const (
	speedtestIOIdleTimeout = 5 * time.Second
	speedtestCloseGrace    = 1 * time.Second
)

func NewSpeedTestHandler(maxConcurrent, maxDurationSec int) *SpeedTestHandler {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	const fallbackRandomSize = 64 * 1024
	handler := &SpeedTestHandler{
		maxConcurrent:  int64(maxConcurrent),
		maxDurationSec: maxDurationSec,
		randomData:     make([]byte, speedtestRandomSize),
		activeByIP:     make(map[string]*speedtestIPCounts),
		uploadBufPool: sync.Pool{
			New: func() any { return make([]byte, uploadReadBufferSize) },
		},
		fallbackRandomPool: sync.Pool{
			New: func() any { return make([]byte, fallbackRandomSize) },
		},
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

func (h *SpeedTestHandler) SetMaxConcurrentPerIP(limit int) {
	if limit < 0 {
		limit = 0
	}
	h.ipMu.Lock()
	h.maxConcurrentPerIP = limit
	h.ipMu.Unlock()
}

func respondSpeedtestError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logging.Warn("speedtest: encode error response", logging.Field{Key: "error", Value: err})
	}
}

func (h *SpeedTestHandler) Download(w http.ResponseWriter, r *http.Request) {
	clientIP := h.resolveClientIP(r)
	if !h.tryAcquireSpeedtestSlot(clientIP, true) {
		drainRequestBody(r)
		respondSpeedtestError(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}
	defer h.releaseSpeedtestSlot(clientIP, true)

	duration, chunkSize, parseErr := parseDownloadParams(r, h.maxDurationSec)
	if parseErr != nil {
		drainRequestBody(r)
		respondSpeedtestError(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set(headerContentType, contentTypeOctetStream)
	w.Header().Set(headerCacheControl, valueNoStore)

	randomSource, release, err := h.resolveRandomSource()
	if err != nil {
		drainRequestBody(r)
		respondSpeedtestError(w, "failed to generate random data", http.StatusInternalServerError)
		return
	}
	defer release()

	streamDownload(w, r, randomSource, chunkSize, duration)
}

func (h *SpeedTestHandler) Upload(w http.ResponseWriter, r *http.Request) {
	defer drainRequestBody(r)

	clientIP := h.resolveClientIP(r)
	if !h.tryAcquireSpeedtestSlot(clientIP, false) {
		respondSpeedtestError(w, "too many concurrent uploads", http.StatusServiceUnavailable)
		return
	}
	defer h.releaseSpeedtestSlot(clientIP, false)

	startTime := time.Now()
	deadline := uploadReadDeadline(startTime, h.maxDurationSec)
	controller := http.NewResponseController(w)
	_ = refreshReadDeadline(controller, deadline)
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

	totalBytes, readFailed := readUploadBody(readCtx, r.Body, controller, deadline, &h.uploadBufPool)
	if readFailed {
		respondSpeedtestError(w, "upload failed", http.StatusInternalServerError)
		return
	}

	writeUploadResponse(w, controller, totalBytes, startTime)
}

func (h *SpeedTestHandler) Ping(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	clientIP := h.resolveClientIP(r)
	isIPv6 := strings.Contains(clientIP, ":")

	w.Header().Set(headerContentType, contentTypeJSON)
	w.Header().Set(headerCacheControl, valueNoStore)
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

func (h *SpeedTestHandler) tryAcquireSpeedtestSlot(clientIP string, isDownload bool) bool {
	counter := &h.activeUploads
	if isDownload {
		counter = &h.activeDownloads
	}
	if atomic.AddInt64(counter, 1) > h.maxConcurrent {
		atomic.AddInt64(counter, -1)
		return false
	}
	if !h.tryAcquirePerIP(clientIP, isDownload) {
		atomic.AddInt64(counter, -1)
		return false
	}
	return true
}

func (h *SpeedTestHandler) releaseSpeedtestSlot(clientIP string, isDownload bool) {
	if isDownload {
		atomic.AddInt64(&h.activeDownloads, -1)
	} else {
		atomic.AddInt64(&h.activeUploads, -1)
	}
	h.releasePerIP(clientIP, isDownload)
}

func (h *SpeedTestHandler) tryAcquirePerIP(clientIP string, isDownload bool) bool {
	if clientIP == "" {
		return true
	}
	h.ipMu.Lock()
	defer h.ipMu.Unlock()
	if h.maxConcurrentPerIP <= 0 {
		return true
	}
	counts := h.activeByIP[clientIP]
	if counts == nil {
		counts = &speedtestIPCounts{}
	}
	current := counts.uploads
	if isDownload {
		current = counts.downloads
	}
	if current >= h.maxConcurrentPerIP {
		return false
	}
	if h.activeByIP[clientIP] == nil {
		h.activeByIP[clientIP] = counts
	}
	if isDownload {
		counts.downloads++
	} else {
		counts.uploads++
	}
	return true
}

func (h *SpeedTestHandler) releasePerIP(clientIP string, isDownload bool) {
	if clientIP == "" {
		return
	}
	h.ipMu.Lock()
	defer h.ipMu.Unlock()
	counts := h.activeByIP[clientIP]
	if counts == nil {
		return
	}
	if isDownload {
		if counts.downloads > 0 {
			counts.downloads--
		}
	} else if counts.uploads > 0 {
		counts.uploads--
	}
	if counts.downloads == 0 && counts.uploads == 0 {
		delete(h.activeByIP, clientIP)
	}
}
