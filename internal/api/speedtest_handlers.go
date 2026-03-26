package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

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
	isIPv6 := strings.IndexByte(clientIP, ':') >= 0

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
