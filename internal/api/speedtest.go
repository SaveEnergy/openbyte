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

func uploadReadDeadline(start time.Time, maxDurationSec int) time.Time {
	if maxDurationSec <= 0 {
		maxDurationSec = 300
	}
	return start.Add(time.Duration(maxDurationSec) * time.Second)
}

func readUploadBody(
	readCtx context.Context,
	body io.Reader,
	controller *http.ResponseController,
	deadline time.Time,
	pool *sync.Pool,
) (totalBytes int64, readFailed bool) {
	buf := getUploadBuf(pool)
	if pool != nil {
		defer pool.Put(buf)
	}
	for {
		select {
		case <-readCtx.Done():
			return totalBytes, false
		default:
		}
		if time.Now().After(deadline) {
			return totalBytes, false
		}
		_ = refreshReadDeadline(controller, deadline)
		n, err := body.Read(buf)
		totalBytes += int64(n)
		if err != nil {
			return totalBytes, !errors.Is(err, io.EOF)
		}
	}
}

func getUploadBuf(pool *sync.Pool) []byte {
	if pool == nil {
		return make([]byte, uploadReadBufferSize)
	}
	pooled := pool.Get()
	if pooled == nil {
		return make([]byte, uploadReadBufferSize)
	}
	if cast, ok := pooled.([]byte); ok && len(cast) >= uploadReadBufferSize {
		return cast[:uploadReadBufferSize]
	}
	return make([]byte, uploadReadBufferSize)
}

func writeUploadResponse(w http.ResponseWriter, controller *http.ResponseController, totalBytes int64, startTime time.Time) {
	elapsed := time.Since(startTime)
	if elapsed <= 0 {
		elapsed = time.Millisecond
	}
	throughputMbps := float64(totalBytes*8) / elapsed.Seconds() / 1_000_000

	w.Header().Set(headerContentType, contentTypeJSON)
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

func parseDownloadParams(r *http.Request, maxDurationSec int) (time.Duration, int, error) {
	duration := 10 * time.Second
	durationRaw := r.URL.Query().Get("duration")
	if d, ok, err := parseOptionalIntInRange(durationRaw, 1, maxDurationSec, "duration must be 1-"+strconv.Itoa(maxDurationSec)); err != nil {
		return 0, 0, err
	} else if ok {
		duration = time.Duration(d) * time.Second
	}

	chunkSize := 1048576
	chunkRaw := r.URL.Query().Get("chunk")
	if c, ok, err := parseOptionalIntInRange(chunkRaw, 65536, 4194304, "chunk must be 65536-4194304"); err != nil {
		return 0, 0, err
	} else if ok {
		chunkSize = c
	}
	return duration, chunkSize, nil
}

func parseOptionalIntInRange(raw string, min, max int, errMessage string) (int, bool, error) {
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, true, errors.New(errMessage)
	}
	return value, true, nil
}

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

func speedtestIdleDeadline(absoluteDeadline time.Time) time.Time {
	idleDeadline := time.Now().Add(speedtestIOIdleTimeout)
	if idleDeadline.After(absoluteDeadline) {
		return absoluteDeadline
	}
	return idleDeadline
}

func refreshReadDeadline(controller *http.ResponseController, absoluteDeadline time.Time) error {
	if controller == nil {
		return nil
	}
	return controller.SetReadDeadline(speedtestIdleDeadline(absoluteDeadline))
}

func refreshWriteDeadline(controller *http.ResponseController, absoluteDeadline time.Time) error {
	if controller == nil {
		return nil
	}
	return controller.SetWriteDeadline(speedtestIdleDeadline(absoluteDeadline))
}
