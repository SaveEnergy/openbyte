package api

import (
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

// BenchmarkClientIPResolverFromRequestXFF is the trusted-proxy + X-Forwarded-For walk (rightmost untrusted).
func BenchmarkClientIPResolverFromRequestXFF(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	resolver := NewClientIPResolver(cfg)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:45632"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 198.51.100.1, 10.0.0.1")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = resolver.FromRequest(req)
	}
}

// BenchmarkClientIPResolverDirectRemote is the fast path when proxy headers are not trusted.
func BenchmarkClientIPResolverDirectRemote(b *testing.B) {
	resolver := NewClientIPResolver(config.DefaultConfig())
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:54321"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = resolver.FromRequest(req)
	}
}
