package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkStaticCacheMiddleware sets cache headers then delegates (GET HTML path).
func BenchmarkStaticCacheMiddleware(b *testing.B) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	h := staticCacheMiddleware(next)
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkIsHTTPSPlain(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if isHTTPS(req) {
			b.Fatal("expected http")
		}
	}
}

func BenchmarkIsHTTPSForwarded(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !isHTTPS(req) {
			b.Fatal("expected https")
		}
	}
}
