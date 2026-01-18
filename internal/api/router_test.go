package api

import "testing"

func TestRouterAllowedOriginWildcard(t *testing.T) {
	router := &Router{
		allowedOrigins: []string{"*.example.com"},
	}

	if !router.isAllowedOrigin("https://foo.example.com") {
		t.Fatalf("expected wildcard origin to be allowed")
	}
}

func TestRouterAllowedOriginHostMatch(t *testing.T) {
	router := &Router{
		allowedOrigins: []string{"foo.example.com"},
	}

	if !router.isAllowedOrigin("https://foo.example.com:8443") {
		t.Fatalf("expected host-only origin to be allowed")
	}
}
