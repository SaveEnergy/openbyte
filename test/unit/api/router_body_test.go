package api_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

func TestRouterDoesNotWaitForUnexpectedSlowGETBody(t *testing.T) {
	router := api.NewRouter(config.DefaultConfig(), nil)
	got := requestWithOpenChunkedBody(t, router.SetupRoutes(), http.MethodGet, healthRoutePath)

	if got.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", got.status, http.StatusOK)
	}
	if !got.close {
		t.Fatal("expected HTTP/1.1 connection with unexpected GET body to close")
	}
}

func TestUploadAtCapacityDoesNotWaitForSlowBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)
	got := requestWithOpenChunkedBody(t, http.HandlerFunc(handler.Upload), http.MethodPost, uploadAPIPath)

	if got.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", got.status, http.StatusServiceUnavailable)
	}
	if got.elapsed > time.Second {
		t.Fatalf("slow-body rejection took %v, want bounded cleanup", got.elapsed)
	}
	if !got.close {
		t.Fatal("expected incomplete upload connection to close")
	}
}

type partialBodyResponse struct {
	status  int
	close   bool
	elapsed time.Duration
}

func requestWithOpenChunkedBody(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
) partialBodyResponse {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set connection deadline: %v", err)
	}

	start := time.Now()
	if _, err := fmt.Fprintf(conn,
		"%s %s HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n",
		method,
		path,
	); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
	if err != nil {
		t.Fatalf("read response while request body remains open: %v", err)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
	return partialBodyResponse{
		status:  resp.StatusCode,
		close:   resp.Close,
		elapsed: time.Since(start),
	}
}
