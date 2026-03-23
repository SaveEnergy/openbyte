package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

func BenchmarkAppendPortIfNonDefault(b *testing.B) {
	host := "192.0.2.10"
	port := "9090"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = appendPortIfNonDefault(host, port)
	}
}

func BenchmarkRequestSchemeHTTP(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = false
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = requestScheme(req, cfg)
	}
}

func BenchmarkRequestSchemeForwardedHTTPS(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.TrustProxyHeaders = true
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = requestScheme(req, cfg)
	}
}

// BenchmarkResponseHostForEndpointPublicHost is the stable URL path when PUBLIC_HOST is set.
func BenchmarkResponseHostForEndpointPublicHost(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.PublicHost = "speed.example.com"
	cfg.TrustProxyHeaders = false
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "localhost:8080"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = responseHostForEndpoint(req, cfg)
	}
}

// BenchmarkResponseHostForEndpointBindUnspecified uses r.Host when bind is wildcard (browser-facing API URL).
func BenchmarkResponseHostForEndpointBindUnspecified(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.BindAddress = "0.0.0.0"
	cfg.Port = "8080"
	cfg.TrustProxyHeaders = false
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "127.0.0.1:8080"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = responseHostForEndpoint(req, cfg)
	}
}
