// Package httpbody provides bounded server request-body cleanup.
package httpbody

import (
	"errors"
	"io"
	"net/http"
	"time"
)

const (
	maxDrainBytes = 64 << 10
	drainTimeout  = 100 * time.Millisecond
)

// Abort stops an unexpected or incomplete request body without waiting for
// the client to finish sending it. HTTP/1.x connections are closed; HTTP/2
// applies the deadline to only the current stream.
func Abort(w http.ResponseWriter, r *http.Request) {
	if !hasBody(r) {
		return
	}
	if r.ProtoMajor == 1 {
		w.Header().Set("Connection", "close")
	}
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now()); err == nil {
		_ = r.Body.Close()
	}
}

// DrainAndClose consumes a small, promptly available body so its connection
// can be reused. Bodies that exceed either bound are aborted instead.
func DrainAndClose(w http.ResponseWriter, r *http.Request) bool {
	if !hasBody(r) {
		return true
	}

	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(drainTimeout)); err != nil {
		Abort(w, r)
		return false
	}

	_, err := io.CopyN(io.Discard, r.Body, maxDrainBytes+1)
	if errors.Is(err, io.EOF) && r.Body.Close() == nil {
		if err := controller.SetReadDeadline(time.Time{}); err == nil {
			return true
		}
	}

	Abort(w, r)
	return false
}

func hasBody(r *http.Request) bool {
	return r != nil && r.Body != nil && r.Body != http.NoBody
}
