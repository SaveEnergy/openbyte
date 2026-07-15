package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/httpbody"
)

func respondSpeedtestError(w http.ResponseWriter, msg string, code int) {
	respondJSON(w, map[string]string{"error": msg}, code)
}

func (h *SpeedTestHandler) Download(w http.ResponseWriter, r *http.Request) {
	clientIP := h.resolveClientIP(r)
	if !h.tryAcquireSpeedtestSlot(clientIP, true) {
		respondSpeedtestError(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}
	defer h.releaseSpeedtestSlot(clientIP, true)

	duration, chunkSize, parseErr := parseDownloadParams(r, h.maxDurationSec)
	if parseErr != nil {
		respondSpeedtestError(w, parseErr.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set(headerContentType, contentTypeOctetStream)
	w.Header().Set(headerCacheControl, valueNoStore)

	streamDownload(w, r, h.randomData, chunkSize, duration)
}

func (h *SpeedTestHandler) Upload(w http.ResponseWriter, r *http.Request) {
	clientIP := h.resolveClientIP(r)
	if !h.tryAcquireSpeedtestSlot(clientIP, false) {
		httpbody.DrainAndClose(w, r)
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
		httpbody.Abort(w, r)
		respondSpeedtestError(w, "upload failed", http.StatusInternalServerError)
		return
	}
	if readCtx.Err() != nil || !time.Now().Before(deadline) {
		httpbody.Abort(w, r)
	} else {
		_ = r.Body.Close()
	}

	writeUploadResponse(w, controller, totalBytes, startTime)
}

func (h *SpeedTestHandler) Ping(w http.ResponseWriter, r *http.Request) {
	h.ping(w, r, "")
}

type pingResponse struct {
	Pong       bool   `json:"pong"`
	Timestamp  int64  `json:"timestamp"`
	ClientIP   string `json:"client_ip"`
	IPv6       bool   `json:"ipv6"`
	ServerName string `json:"server_name,omitempty"`
}

func (h *SpeedTestHandler) ping(w http.ResponseWriter, r *http.Request, serverName string) {
	clientIP := h.resolveClientIP(r)
	isIPv6 := strings.IndexByte(clientIP, ':') >= 0

	w.Header().Set(headerContentType, contentTypeJSON)
	w.Header().Set(headerCacheControl, valueNoStore)
	if r.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(pingResponse{
		Pong:       true,
		Timestamp:  time.Now().UnixMilli(),
		ClientIP:   clientIP,
		IPv6:       isIPv6,
		ServerName: serverName,
	}); err != nil {
		slog.Warn("speedtest: encode ping response", "error", err)
	}
}
