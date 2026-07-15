package httpbody

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func newDeadlineRecorder() *deadlineRecorder {
	return &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (w *deadlineRecorder) SetReadDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

type trackingBody struct {
	data   []byte
	offset int
	reads  int
	closed bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	b.reads++
	if b.offset == len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

func TestDrainAndCloseDrainsFiniteBodyAndResetsDeadline(t *testing.T) {
	body := &trackingBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = body
	req.ContentLength = int64(len(body.data))
	w := newDeadlineRecorder()

	if !DrainAndClose(w, req) {
		t.Fatal("expected finite body to be reusable")
	}
	if body.offset != len(body.data) || !body.closed {
		t.Fatalf("body offset=%d closed=%v, want %d/true", body.offset, body.closed, len(body.data))
	}
	if len(w.deadlines) != 2 || w.deadlines[0].IsZero() || !w.deadlines[1].IsZero() {
		t.Fatalf("deadlines = %v, want bounded deadline then reset", w.deadlines)
	}
	if got := w.Header().Get("Connection"); got != "" {
		t.Fatalf("Connection = %q, want reusable connection", got)
	}
}

func TestAbortClosesBodyWithoutReading(t *testing.T) {
	body := &trackingBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = body
	req.ContentLength = int64(len(body.data))
	w := newDeadlineRecorder()

	Abort(w, req)

	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if !body.closed {
		t.Fatal("expected body to be closed")
	}
	if got := w.Header().Get("Connection"); got != "close" {
		t.Fatalf("Connection = %q, want close", got)
	}
	if len(w.deadlines) != 1 || w.deadlines[0].After(time.Now()) {
		t.Fatalf("deadlines = %v, want immediate abort deadline", w.deadlines)
	}
}

func TestDrainAndCloseAbortsOversizedBodyAtByteBound(t *testing.T) {
	body := &trackingBody{data: bytes.Repeat([]byte("x"), maxDrainBytes+2)}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = body
	req.ContentLength = int64(len(body.data))
	w := newDeadlineRecorder()

	if DrainAndClose(w, req) {
		t.Fatal("expected oversized body to be aborted")
	}
	if body.offset != maxDrainBytes+1 {
		t.Fatalf("body bytes read = %d, want %d", body.offset, maxDrainBytes+1)
	}
	if !body.closed {
		t.Fatal("expected body to be closed")
	}
	if got := w.Header().Get("Connection"); got != "close" {
		t.Fatalf("Connection = %q, want close", got)
	}
	if len(w.deadlines) != 2 || w.deadlines[1].After(time.Now()) {
		t.Fatalf("deadlines = %v, want final abort deadline", w.deadlines)
	}
}

func TestDrainAndCloseWithoutDeadlineSupportDoesNotRead(t *testing.T) {
	body := &trackingBody{data: []byte("finite")}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = body
	req.ContentLength = int64(len(body.data))
	w := httptest.NewRecorder()

	if DrainAndClose(w, req) {
		t.Fatal("expected unsupported deadline writer to abort")
	}
	if body.reads != 0 {
		t.Fatalf("body reads = %d, want 0", body.reads)
	}
	if body.closed {
		t.Fatal("expected unsupported writer to leave body cleanup to the server")
	}
	if got := w.Header().Get("Connection"); got != "close" {
		t.Fatalf("Connection = %q, want close", got)
	}
}

func TestDrainAndClosePreservesHTTP1ConnectionReuse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && !DrainAndClose(w, r) {
			t.Error("finite body unexpectedly aborted")
		}
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	conn := dialServer(t, srv.URL)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writeRequest(t, conn, "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 4\r\n\r\ndata")
	readResponse(t, reader, http.MethodPost)

	time.Sleep(drainTimeout + 50*time.Millisecond)
	writeRequest(t, conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n")
	readResponse(t, reader, http.MethodGet)
}

func TestDrainAndCloseTimesOutSlowHTTP1Body(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		DrainAndClose(w, r)
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusUnsupportedMediaType)
		_, _ = io.WriteString(w, "no")
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	conn := dialServer(t, srv.URL)
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	writeRequest(t, conn, "POST / HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n")

	resp := readResponse(t, bufio.NewReader(conn), http.MethodPost)
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnsupportedMediaType)
	}
	if !resp.Close {
		t.Fatal("expected slow-body connection to close")
	}
}

func TestHTTP2BodyCleanupResetsOnlyCurrentStream(t *testing.T) {
	tests := []struct {
		name    string
		cleanup func(http.ResponseWriter, *http.Request)
	}{
		{name: "abort", cleanup: Abort},
		{name: "drain timeout", cleanup: func(w http.ResponseWriter, r *http.Request) {
			DrainAndClose(w, r)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testHTTP2BodyCleanup(t, tc.cleanup)
		})
	}
}

func testHTTP2BodyCleanup(t *testing.T, cleanup func(http.ResponseWriter, *http.Request)) {
	t.Helper()
	var connections atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanup(w, r)
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	}))
	srv.EnableHTTP2 = true
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	reader, writer := io.Pipe()
	req, err := http.NewRequest(http.MethodGet, srv.URL, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	type result struct {
		resp *http.Response
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, requestErr := srv.Client().Do(req)
		done <- result{resp: resp, err: requestErr}
	}()
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("write request body: %v", err)
	}

	var first result
	select {
	case first = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP/2 response blocked on request body")
	}
	_ = writer.Close()
	if first.err != nil {
		t.Fatalf("first request: %v", first.err)
	}
	readAndClose(t, first.resp)
	if first.resp.ProtoMajor != 2 {
		t.Fatalf("protocol = %s, want HTTP/2", first.resp.Proto)
	}

	second, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	readAndClose(t, second)
	if got := connections.Load(); got != 1 {
		t.Fatalf("connections = %d, want HTTP/2 connection reuse", got)
	}
}

func dialServer(t *testing.T, rawURL string) net.Conn {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	return conn
}

func writeRequest(t *testing.T, w io.Writer, request string) {
	t.Helper()
	if _, err := io.Copy(w, strings.NewReader(request)); err != nil {
		t.Fatalf("write request: %v", err)
	}
}

func readResponse(t *testing.T, reader *bufio.Reader, method string) *http.Response {
	t.Helper()
	resp, err := http.ReadResponse(reader, &http.Request{Method: method})
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	readAndClose(t, resp)
	return resp
}

func readAndClose(t *testing.T, resp *http.Response) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
	if string(body) != "ok" && string(body) != "no" {
		t.Fatalf("response body = %q", body)
	}
}
