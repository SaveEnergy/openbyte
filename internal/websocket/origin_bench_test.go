package websocket

import "testing"

// BenchmarkWebsocketIsAllowedOriginExact exercises WS CheckOrigin allow-list matching (exact origin).
func BenchmarkWebsocketIsAllowedOriginExact(b *testing.B) {
	s := NewServer()
	s.SetAllowedOrigins([]string{
		"https://app.example.com",
		"https://partner.example.org",
		"https://admin.internal",
	})
	origin := "https://app.example.com"
	host := "api.example.com:443"

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if !s.isAllowedOrigin(origin, host) {
			b.Fatal("expected allowed")
		}
	}
}
