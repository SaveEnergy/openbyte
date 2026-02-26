package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

const (
	versionEndpoint = "/api/v1/version"
	serversEndpoint = "/api/v1/servers"
)

func TestGetVersion(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	handler.SetVersion("1.2.3")

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode version response: %v", err)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", resp.Version)
	}
}

func TestGetVersionDefault(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	var resp api.VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode default version response: %v", err)
	}
	if resp.Version != "dev" {
		t.Errorf("default version = %q, want dev", resp.Version)
	}
}

func TestGetVersionDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}

	req := httptest.NewRequest(http.MethodGet, versionEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()
	handler.GetVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetServers(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.ServerName = "Test Server"
	handler.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, serversEndpoint, nil)
	req.Host = "testhost:8080"
	rec := httptest.NewRecorder()
	handler.GetServers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	var resp api.ServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode servers response: %v", err)
	}
	if len(resp.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(resp.Servers))
	}
	if resp.Servers[0].Name != "Test Server" {
		t.Errorf("server name = %q, want Test Server", resp.Servers[0].Name)
	}
	if resp.Servers[0].Health != "healthy" {
		t.Errorf("health = %q, want healthy", resp.Servers[0].Health)
	}
}

func TestGetServersIgnoresRequestHostWhenProxyUntrusted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = "8080"
	cfg.TrustProxyHeaders = false
	handler.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, serversEndpoint, nil)
	req.Host = "evil.example:9999"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.GetServers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	var resp api.ServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode servers response: %v", err)
	}
	if len(resp.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(resp.Servers))
	}
	if resp.Servers[0].APIEndpoint != "http://127.0.0.1:8080" {
		t.Fatalf("api endpoint = %q, want %q", resp.Servers[0].APIEndpoint, "http://127.0.0.1:8080")
	}
}

func TestGetServersPreservesProxyPort(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	handler.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, serversEndpoint, nil)
	req.Host = "proxy.example.com:8443"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.GetServers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}

	var resp api.ServersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode servers response: %v", err)
	}
	if len(resp.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(resp.Servers))
	}
	want := "https://proxy.example.com:8443"
	if resp.Servers[0].APIEndpoint != want {
		t.Errorf("api_endpoint = %q, want %q", resp.Servers[0].APIEndpoint, want)
	}
}

func TestGetServersRejectsWrongMethodDrainsBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	handler := api.NewHandler(mgr)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodPost, serversEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetServers(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusMethodNotAllowed)
	}
	assertTrackingBodyDrained(t, tb)
}
