package api_test

import (
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
)

const (
	clientIPMethod           = "GET"
	clientIPURL              = "http://example.com"
	localhostWithPort        = "127.0.0.1:1234"
	clientLoopbackIP         = "127.0.0.1"
	forwardedClientIP        = "203.0.113.10"
	realClientIP             = "10.0.0.1"
	realIPHeaderValue        = "198.51.100.5"
	headerForwardedFor       = "X-Forwarded-For"
	headerRealIP             = "X-Real-IP"
	trustedLoopbackCIDR      = "127.0.0.0/8"
	trustedPrivateCIDR       = "10.0.0.0/8"
	clientIPWantFmt          = "client ip = %s, want %s"
	clientIPRightmostWantFmt = "client ip = %s, want %s (rightmost untrusted)"
)

func TestClientIPResolverTrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{trustedLoopbackCIDR, trustedPrivateCIDR}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	// Rightmost-untrusted: skip 10.0.0.1 (trusted), return 203.0.113.10 (real client)
	req.Header.Set(headerForwardedFor, forwardedClientIP+", "+realClientIP)

	ip := resolver.FromRequest(req)
	if ip != forwardedClientIP {
		t.Fatalf(clientIPWantFmt, ip, forwardedClientIP)
	}
}

func TestClientIPResolverRightmostUntrusted(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{trustedLoopbackCIDR}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	// Attacker prepends spoofed IP; rightmost-untrusted returns 10.0.0.1 (real client)
	req.Header.Set(headerForwardedFor, "1.2.3.4, "+realClientIP)

	ip := resolver.FromRequest(req)
	if ip != realClientIP {
		t.Fatalf(clientIPRightmostWantFmt, ip, realClientIP)
	}
}

func TestClientIPResolverUntrustedProxy(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{trustedPrivateCIDR}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	req.Header.Set(headerForwardedFor, forwardedClientIP)

	ip := resolver.FromRequest(req)
	if ip != clientLoopbackIP {
		t.Fatalf(clientIPWantFmt, ip, clientLoopbackIP)
	}
}

func TestClientIPResolverFallbackToRealIP(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{trustedLoopbackCIDR}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	req.Header.Set(headerRealIP, realIPHeaderValue)

	ip := resolver.FromRequest(req)
	if ip != realIPHeaderValue {
		t.Fatalf(clientIPWantFmt, ip, realIPHeaderValue)
	}
}

func TestClientIPResolverXFFOnlyTrustedNoXRealIPFallback(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{trustedLoopbackCIDR, trustedPrivateCIDR}

	resolver := api.NewClientIPResolver(cfg)
	req := httptest.NewRequest(clientIPMethod, clientIPURL, nil)
	req.RemoteAddr = localhostWithPort
	// XFF contains only trusted IPs; X-Real-IP could be attacker-spoofed — must not use it.
	req.Header.Set(headerForwardedFor, realClientIP)
	req.Header.Set(headerRealIP, "1.2.3.4")

	ip := resolver.FromRequest(req)
	if ip != clientLoopbackIP {
		t.Fatalf("when XFF has only trusted hops, must fall back to remoteAddr, not X-Real-IP: got %s, want %s", ip, clientLoopbackIP)
	}
}
