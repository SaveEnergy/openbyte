package registry_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/registry"
)

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func newRegistryClientConfig(url string, interval time.Duration) *config.Config {
	cfg := config.DefaultConfig()
	cfg.RegistryEnabled = true
	cfg.RegistryURL = url
	cfg.RegistryInterval = interval
	cfg.ServerID = "srv-test"
	cfg.ServerName = "srv-test"
	cfg.ServerLocation = "test"
	cfg.Port = "8080"
	cfg.TCPTestPort = 8081
	cfg.UDPTestPort = 8082
	cfg.CapacityGbps = 10
	cfg.MaxConcurrentTests = 5
	return cfg
}

func TestClientStartHeartbeatAndStopLifecycle(t *testing.T) {
	var postCount atomic.Int64
	var putCount atomic.Int64
	var deleteCount atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCount.Add(1)
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"registered"}`))
		case http.MethodPut:
			putCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		case http.MethodDelete:
			deleteCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"deregistered"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	cfg := newRegistryClientConfig(srv.URL, 20*time.Millisecond)
	client := registry.NewClient(cfg, logging.NewLogger("test"))

	if err := client.Start(func() int { return 3 }); err != nil {
		t.Fatalf("start: %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool { return postCount.Load() >= 1 }, "expected register call")
	waitFor(t, 800*time.Millisecond, func() bool { return putCount.Load() >= 1 }, "expected at least one heartbeat call")

	client.Stop()
	waitFor(t, 500*time.Millisecond, func() bool { return deleteCount.Load() == 1 }, "expected one deregister call on stop")

	// Must be safe to call more than once.
	client.Stop()
}

func TestClientHeartbeatReRegistersOnNotFound(t *testing.T) {
	var postCount atomic.Int64
	var putCount atomic.Int64
	var deleteCount atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putCount.Add(1)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case http.MethodPost:
			postCount.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"registered"}`))
		case http.MethodDelete:
			deleteCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"deregistered"}`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	cfg := newRegistryClientConfig(srv.URL, 20*time.Millisecond)
	client := registry.NewClient(cfg, logging.NewLogger("test"))

	if err := client.Start(func() int { return 2 }); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	defer client.Stop()

	waitFor(t, 500*time.Millisecond, func() bool { return putCount.Load() >= 1 }, "expected heartbeat PUT call")
	waitFor(t, 500*time.Millisecond, func() bool { return postCount.Load() >= 2 }, "expected re-register POST call after 404")

	client.Stop()
	waitFor(t, 500*time.Millisecond, func() bool { return deleteCount.Load() == 1 }, "expected deregister on stop")
}

func TestClientStartIdempotent(t *testing.T) {
	var postCount atomic.Int64
	var deleteCount atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCount.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"registered"}`))
		case http.MethodDelete:
			deleteCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"deregistered"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer srv.Close()

	cfg := newRegistryClientConfig(srv.URL, time.Hour)
	client := registry.NewClient(cfg, logging.NewLogger("test"))

	if err := client.Start(func() int { return 1 }); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	if err := client.Start(func() int { return 1 }); err != nil {
		t.Fatalf("start 2: %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool { return postCount.Load() == 1 }, "expected single register call")
	client.Stop()
	waitFor(t, 500*time.Millisecond, func() bool { return deleteCount.Load() == 1 }, "expected single deregister call")
}
