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
	handler := api.NewHandler()
	router := api.NewRouter(handler, config.DefaultConfig())
	srv := httptest.NewServer(router.SetupRoutes())
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

	_, err = fmt.Fprintf(conn,
		"GET %s HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n",
		versionAPIPath,
	)
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatalf("read response while request body remains open: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !resp.Close {
		t.Fatal("expected HTTP/1.1 connection with unexpected GET body to close")
	}
}

func TestUploadAtCapacityDoesNotWaitForSlowBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)
	srv := httptest.NewServer(http.HandlerFunc(handler.Upload))
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
	_, err = fmt.Fprint(conn,
		"POST /api/v1/upload HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n",
	)
	if err != nil {
		t.Fatalf("write request: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatalf("read rejection while upload body remains open: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("slow-body rejection took %v, want bounded cleanup", elapsed)
	}
	if !resp.Close {
		t.Fatal("expected incomplete upload connection to close")
	}
}
