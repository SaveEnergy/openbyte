package api

import (
	"net/http"
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
	readCtx := r.Context()
	if requestDeadline, ok := readCtx.Deadline(); ok && requestDeadline.Before(deadline) {
		deadline = requestDeadline
	}
	controller := http.NewResponseController(w)
	_ = refreshReadDeadline(controller, deadline)
	// Unblock a stalled body even when the ResponseWriter cannot set deadlines.
	// Successful uploads stop the timer without creating a waiting goroutine.
	closeTimer := time.AfterFunc(time.Until(deadline), func() { _ = r.Body.Close() })
	defer closeTimer.Stop()

	totalBytes, readFailed := h.readUploadBody(readCtx, r.Body, controller, deadline)
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
	ClientIP   string `json:"client_ip"`
	ServerName string `json:"server_name,omitempty"`
}

func (h *SpeedTestHandler) ping(w http.ResponseWriter, r *http.Request, serverName string) {
	w.Header().Set(headerCacheControl, valueNoStore)
	if r.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	respondJSON(w, pingResponse{
		ClientIP:   h.resolveClientIP(r),
		ServerName: serverName,
	}, http.StatusOK)
}
