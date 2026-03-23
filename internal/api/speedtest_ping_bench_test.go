package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

// BenchmarkSpeedTestPing is the /api/v1/ping JSON response path (frequent health-style probe).
func BenchmarkSpeedTestPing(b *testing.B) {
	cfg := config.DefaultConfig()
	h := NewSpeedTestHandler(cfg.MaxConcurrentHTTP(), 300)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	req.RemoteAddr = "192.0.2.10:54321"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		w := httptest.NewRecorder()
		h.Ping(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("status=%d", w.Code)
		}
	}
}
