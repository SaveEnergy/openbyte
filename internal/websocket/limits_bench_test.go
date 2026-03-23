package websocket

import (
	"net/http/httptest"
	"testing"
)

// BenchmarkReserveReleaseConnectionSlot is per-upgrade accounting (limits + per-IP map).
func BenchmarkReserveReleaseConnectionSlot(b *testing.B) {
	s := NewServer()
	s.SetConnectionLimits(100_000, 10_000)
	ip := "192.0.2.77"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !s.reserveConnectionSlot(ip) {
			b.Fatal("expected slot")
		}
		s.releaseConnectionSlot(ip)
	}
}

// BenchmarkWebsocketClientIPHeader prefers X-OpenByte-Client-IP when set (internal header).
func BenchmarkWebsocketClientIPHeader(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-OpenByte-Client-IP", "203.0.113.50")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = websocketClientIP(req)
	}
}

// BenchmarkWebsocketClientIPRemoteAddr falls back to StripHostPort(RemoteAddr).
func BenchmarkWebsocketClientIPRemoteAddr(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.10:54321"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = websocketClientIP(req)
	}
}
