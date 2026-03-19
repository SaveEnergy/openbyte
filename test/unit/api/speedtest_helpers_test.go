package api_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

const (
	speedtestStatusFmt                 = "status = %d, want %d"
	octetStreamType                    = "application/octet-stream"
	jsonContentType                    = "application/json"
	noStoreCacheControl                = "no-store"
	downloadEndpointBase               = "http://example.com/api/v1/download"
	uploadEndpoint                     = "http://example.com/api/v1/upload"
	pingEndpoint                       = "http://example.com/api/v1/ping"
	statusServiceUnavailable           = http.StatusServiceUnavailable
	statusBadRequest                   = http.StatusBadRequest
	statusInternalServerErr            = http.StatusInternalServerError
	speedtestQueryDur1Chunk            = "?duration=1&chunk=65536"
	speedtestQueryDur60Chunk           = "?duration=60&chunk=65536"
	speedtestQueryDur5Chunk            = "?duration=5&chunk=65536"
	speedtestQueryDur10Chunk           = "?duration=10&chunk=65536"
	speedtestQueryDurZero              = "?duration=0"
	speedtestQueryChunkBad             = "?chunk=bad"
	speedtestQueryChunkABC             = "?chunk=abc"
	speedtestWaitTimeout               = 2 * time.Second
	pingClientIPKey                    = "client_ip"
	pingIPv6Key                        = "ipv6"
	pingClientIPv4Want                 = "203.0.113.10"
	pingClientIPv6Want                 = "2001:db8::1"
	pingIPv6FalseMsg                   = "ipv6 = %v, want false"
	pingIPv6TrueMsg                    = "ipv6 = %v, want true"
	speedtestContentTypeKey            = "Content-Type"
	speedtestCacheControlKey           = "Cache-Control"
	speedtestDecodeRespFmt             = "decode response: %v"
	speedtestExpectBodyDrained         = "expected body to be drained"
	speedtestExpectBodyClosed          = "expected body to be closed"
	speedtestExpectReqBodyDrained503   = "expected request body to be drained before returning 503"
	speedtestExpectReqBodyClosed       = "expected request body to be closed"
	speedtestWaitDownloadStartTimeout  = "timed out waiting for download goroutine to start"
	speedtestWaitUploadStartTimeout    = "timed out waiting for upload goroutine to start"
	speedtestExpected503AtLimitFmt     = "expected 503 when at limit, got %d"
	speedtestExpected200AfterCancelFmt = "expected 200 after cancel freed slots, got %d"
	speedtestURLStatusFmt              = "url=%s status = %d, want %d"
	speedtestURLBodyDrainedFmt         = "url=%s expected body to be drained"
	speedtestURLBodyClosedFmt          = "url=%s expected body to be closed"
	speedtestDurationTooHighFmt        = "duration=10 with max=5: status = %d, want 400"
	speedtestChunkABCFmt               = "chunk=abc: status = %d, want 400"
)

// signalWriter wraps httptest.ResponseRecorder and signals a channel on first Write,
// indicating the handler has started (and incremented its concurrency counter).
type signalWriter struct {
	*httptest.ResponseRecorder
	started chan struct{}
	once    sync.Once
}

func (sw *signalWriter) Write(b []byte) (int, error) {
	sw.once.Do(func() { close(sw.started) })
	return sw.ResponseRecorder.Write(b)
}

func (sw *signalWriter) WriteHeader(code int) {
	sw.once.Do(func() { close(sw.started) })
	sw.ResponseRecorder.WriteHeader(code)
}

// signalReader wraps a Reader and signals on the first Read call.
type signalReader struct {
	io.ReadCloser
	started chan struct{}
	once    sync.Once
}

func (sr *signalReader) Read(p []byte) (int, error) {
	sr.once.Do(func() { close(sr.started) })
	return sr.ReadCloser.Read(p)
}

type errReader struct {
	readOnce bool
}

func (e *errReader) Read(p []byte) (int, error) {
	if !e.readOnce {
		e.readOnce = true
		p[0] = 'x'
		return 1, nil
	}
	return 0, errors.New("read failure")
}

type trackingUploadBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (tb *trackingUploadBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.offset >= len(tb.data) {
		return 0, io.EOF
	}
	n := copy(p, tb.data[tb.offset:])
	tb.offset += n
	return n, nil
}

func (tb *trackingUploadBody) Close() error {
	tb.closed = true
	return nil
}

type failingTrackingBody struct {
	reads  int
	closed bool
}

func (tb *failingTrackingBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.reads == 1 {
		p[0] = 'x'
		return 1, nil
	}
	return 0, errors.New("read failure")
}

func (tb *failingTrackingBody) Close() error {
	tb.closed = true
	return nil
}

type blockingUploadBody struct {
	closed chan struct{}
	once   sync.Once
}

func (b *blockingUploadBody) Read(_ []byte) (int, error) {
	<-b.closed
	return 0, io.ErrClosedPipe
}

func (b *blockingUploadBody) Close() error {
	b.once.Do(func() {
		close(b.closed)
	})
	return nil
}
