package api

import (
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

func BenchmarkRouterResolveClientIPNoResolver(b *testing.B) {
	r := &Router{}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:54321"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = r.resolveClientIP(req)
	}
}

func BenchmarkRouterResolveClientIPWithResolver(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	r := &Router{
		clientIPResolver: NewClientIPResolver(cfg),
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:443"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = r.resolveClientIP(req)
	}
}
