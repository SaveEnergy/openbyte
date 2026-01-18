package api

import (
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

func TestClientIPResolver_TrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8"}

	resolver := NewClientIPResolver(cfg)
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")

	ip := resolver.FromRequest(req)
	if ip != "203.0.113.10" {
		t.Fatalf("client ip = %s, want 203.0.113.10", ip)
	}
}

func TestClientIPResolver_UntrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}

	resolver := NewClientIPResolver(cfg)
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	ip := resolver.FromRequest(req)
	if ip != "127.0.0.1" {
		t.Fatalf("client ip = %s, want 127.0.0.1", ip)
	}
}

func TestClientIPResolver_FallbackToRealIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"127.0.0.0/8"}

	resolver := NewClientIPResolver(cfg)
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Real-IP", "198.51.100.5")

	ip := resolver.FromRequest(req)
	if ip != "198.51.100.5" {
		t.Fatalf("client ip = %s, want 198.51.100.5", ip)
	}
}
