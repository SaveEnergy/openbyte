package httptransfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingBody struct {
	reader io.Reader
	read   int
	closed bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.read += n
	return n, err
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

type partialErrorBody struct {
	sent   bool
	closed bool
	err    error
}

func (b *partialErrorBody) Read(p []byte) (int, error) {
	if !b.sent {
		b.sent = true
		return copy(p, "abc"), nil
	}
	return 0, b.err
}

func (b *partialErrorBody) Close() error {
	b.closed = true
	return nil
}

func TestDownloadReportsPartialBytesAndReadError(t *testing.T) {
	wantErr := errors.New("read failed")
	body := &partialErrorBody{err: wantErr}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", req.Method)
		}
		if got := req.Header.Get("Accept-Encoding"); got != "identity" {
			t.Fatalf("Accept-Encoding = %q, want identity", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: body}, nil
	})}

	var bytesRead int
	err := Download(context.Background(), client, "http://example.test/download", make([]byte, 8), func(n int) {
		bytesRead += n
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if bytesRead != 3 {
		t.Fatalf("bytes read = %d, want 3", bytesRead)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestUploadNonOKDrainsAndReturnsStatus(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader("busy")}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if got := req.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("Content-Type = %q, want application/octet-stream", got)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read payload: %v", err)
		}
		if string(payload) != "payload" {
			t.Fatalf("payload = %q, want payload", payload)
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Status:     "201 Created",
			Header:     http.Header{"Retry-After": []string{"7"}},
			Body:       body,
		}, nil
	})}

	err := Upload(context.Background(), client, "http://example.test/upload", []byte("payload"))
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want StatusError", err)
	}
	if statusErr.Code != http.StatusCreated || statusErr.Status != "201 Created" || statusErr.RetryAfter != "7" {
		t.Fatalf("status error = %+v", statusErr)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
	if body.read != len("busy") {
		t.Fatalf("response bytes read = %d, want %d", body.read, len("busy"))
	}
}

func TestMeasurePingDrainsAndCloses(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader(`{"pong":true}`)}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: body}, nil
	})}

	duration, ok := MeasurePing(client, mustRequest(t), time.Now().Add(-time.Millisecond))
	if !ok || duration <= 0 {
		t.Fatalf("MeasurePing() = %s, %t", duration, ok)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
	if body.read != len(`{"pong":true}`) {
		t.Fatalf("response bytes read = %d", body.read)
	}
}

func mustRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
