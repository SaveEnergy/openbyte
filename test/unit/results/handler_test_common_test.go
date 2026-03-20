package results_test

import (
	"errors"
	"io"
	"net/http"
)

type failingResponseWriter struct {
	header http.Header
	status int
	writes int
}

const (
	resultsPath         = "/api/v1/results"
	resultsDBName       = "results.db"
	newStoreFmt         = "new store: %v"
	contentTypeHeader   = "Content-Type"
	applicationJSON     = "application/json"
	cacheControlHeader  = "Cache-Control"
	cacheNoStore        = "no-store"
	statusCodeWantFmt   = "status = %d, want %d"
	cacheControlWantFmt = "cache-control = %q, want %q"
	plainTextType       = "text/plain"
	abcResultID         = "abc12345"
	invalidResultID     = "bad-id"
	missingResultID     = "missing1"
	saveErrorMsg        = "failed to save result"
	internalErrorMsg    = "internal error"
	bodyTooLargeMsg     = "request body too large"
	decodeResponseFmt   = "decode response: %v"
	errorWantFmt        = "error = %q, want %q"
	sampleResultPayload = `{
		"download_mbps": 100,
		"upload_mbps": 50,
		"latency_ms": 10,
		"jitter_ms": 1,
		"loaded_latency_ms": 12,
		"bufferbloat_grade": "A",
		"ipv4": "203.0.113.10",
		"ipv6": "",
		"server_name": "test"
	}`
)

func (fw *failingResponseWriter) Header() http.Header {
	if fw.header == nil {
		fw.header = make(http.Header)
	}
	return fw.header
}

func (fw *failingResponseWriter) WriteHeader(code int) {
	fw.status = code
}

func (fw *failingResponseWriter) Write(_ []byte) (int, error) {
	fw.writes++
	return 0, errors.New("write failed")
}

type trackingBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (tb *trackingBody) Read(p []byte) (int, error) {
	tb.reads++
	if tb.offset >= len(tb.data) {
		return 0, io.EOF
	}
	n := copy(p, tb.data[tb.offset:])
	tb.offset += n
	return n, nil
}

func (tb *trackingBody) Close() error {
	tb.closed = true
	return nil
}
