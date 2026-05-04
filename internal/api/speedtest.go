package api

import (
	"crypto/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

type SpeedTestHandler struct {
	activeDownloads    int64
	activeUploads      int64
	maxConcurrent      int64
	maxConcurrentPerIP atomic.Int64
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
	speedtestIOIdleTimeout         = 5 * time.Second
	speedtestDeadlineRefreshPeriod = time.Second
	speedtestCloseGrace            = 1 * time.Second
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
			New: func() any { return newUploadBuffer() },
		},
		fallbackRandomPool: sync.Pool{
			New: func() any { return newSpeedtestBuffer(fallbackRandomSize) },
		},
	}
	if _, err := rand.Read(handler.randomData); err != nil {
		logging.Warn("speedtest: random data init failed, using per-request random",
			logging.Field{Key: "error", Value: err})
		handler.randomData = nil
	}
	return handler
}

func newUploadBuffer() *[]byte {
	return newSpeedtestBuffer(uploadReadBufferSize)
}

func newSpeedtestBuffer(size int) *[]byte {
	buf := make([]byte, size)
	return &buf
}

func (h *SpeedTestHandler) SetClientIPResolver(resolver *ClientIPResolver) {
	h.clientIPResolver = resolver
}

func (h *SpeedTestHandler) SetMaxConcurrentPerIP(limit int) {
	if limit < 0 {
		limit = 0
	}
	h.maxConcurrentPerIP.Store(int64(limit))
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
	if h.maxConcurrentPerIP.Load() <= 0 {
		return true
	}
	h.ipMu.Lock()
	defer h.ipMu.Unlock()
	limit := int(h.maxConcurrentPerIP.Load())
	if limit <= 0 {
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
	if current >= limit {
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
