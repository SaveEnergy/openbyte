package websocket

import "testing"

func TestServerAllowedOriginWildcard(t *testing.T) {
	s := NewServer()
	s.SetAllowedOrigins([]string{"*.example.com"})

	if !s.isAllowedOrigin("https://foo.example.com", "foo.example.com") {
		t.Fatalf("expected wildcard origin to be allowed")
	}
}

func TestServerAllowedOriginHostMatch(t *testing.T) {
	s := NewServer()
	s.SetAllowedOrigins([]string{"foo.example.com"})

	if !s.isAllowedOrigin("https://foo.example.com:8443", "foo.example.com:8443") {
		t.Fatalf("expected host-only origin to be allowed")
	}
}
