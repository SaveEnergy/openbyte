package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientHasDefaultHTTPTimeout(t *testing.T) {
	c := New("http://localhost:8080")
	if c.httpClient == nil {
		t.Fatal("http client should be initialized")
	}
	if c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("default http client timeout = %v, want %v", c.httpClient.Timeout, defaultHTTPTimeout)
	}
}

func TestUploadMeasuredAllowsSlowFirstResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(pathUpload, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		time.Sleep(4*time.Second + 250*time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL)
	mbps, totalBytes, ok := c.uploadMeasured(context.Background(), 1)
	if !ok {
		t.Fatalf("uploadMeasured ok = false, mbps=%f totalBytes=%d", mbps, totalBytes)
	}
	if totalBytes == 0 {
		t.Fatal("expected uploadMeasured to count the completed upload")
	}
}
