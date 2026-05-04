package api_test

import (
	"io"
	"testing"
)

const (
	applicationJSON   = "application/json"
	contentTypeHeader = "Content-Type"
	statusCodeWantFmt = "status = %d, want %d"
)

type trackingBody struct {
	data   []byte
	closed bool
}

func (tb *trackingBody) Read(p []byte) (int, error) {
	if len(tb.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, tb.data)
	tb.data = tb.data[n:]
	return n, nil
}

func (tb *trackingBody) Close() error {
	tb.closed = true
	return nil
}

func assertTrackingBodyDrained(t *testing.T, tb *trackingBody) {
	t.Helper()
	if len(tb.data) != 0 {
		t.Fatal("expected request body to be drained")
	}
	if !tb.closed {
		t.Fatal("expected request body to be closed")
	}
}
