package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	client "github.com/saveenergy/openbyte/cmd/client"
)

func TestCancelStreamUsesDetachedContextWhenParentCanceled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := client.CancelStream(ctx, server.URL, "stream-id", "")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success with detached cancel context, got: %v", err)
	}
	if !called {
		t.Fatal("request should reach server even when parent context is canceled")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("CancelStream took too long: %v", elapsed)
	}
}

func TestCancelStreamSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"cancelled"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.CancelStream(ctx, server.URL, "stream-id", ""); err != nil {
		t.Fatalf("CancelStream: %v", err)
	}
}

func TestCancelStreamServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.CancelStream(ctx, server.URL, "stream-id", ""); err == nil {
		t.Fatal("expected error on non-2xx cancel response")
	}
}
