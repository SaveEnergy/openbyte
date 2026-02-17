package api_test

import (
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const (
	clientIPMethod    = "GET"
	clientIPURL       = "http://example.com"
	localhostWithPort = "127.0.0.1:1234"
	clientLoopbackIP  = "127.0.0.1"
)

func TestClientIPResolverTrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8", "10.0.0.0/8"}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	// Rightmost-untrusted: skip 10.0.0.1 (trusted), return 203.0.113.10 (real client)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	ip := resolver.FromRequest(req)
	if ip != "203.0.113.10" {
		t.Fatalf("client ip = %s, want 203.0.113.10", ip)
	}
}

func TestClientIPResolverRightmostUntrusted(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8"}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	// Attacker prepends spoofed IP; rightmost-untrusted returns 10.0.0.1 (real client)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")

	ip := resolver.FromRequest(req)
	if ip != "10.0.0.1" {
		t.Fatalf("client ip = %s, want 10.0.0.1 (rightmost untrusted)", ip)
	}
}

func TestClientIPResolverUntrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	ip := resolver.FromRequest(req)
	if ip != clientLoopbackIP {
		t.Fatalf("client ip = %s, want %s", ip, clientLoopbackIP)
	}
}

func TestClientIPResolverFallbackToRealIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8"}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	req.Header.Set("X-Real-IP", "198.51.100.5")

	ip := resolver.FromRequest(req)
	if ip != "198.51.100.5" {
		t.Fatalf("client ip = %s, want 198.51.100.5", ip)
	}
}
