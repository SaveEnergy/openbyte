package websocket_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	obytewebsocket "github.com/saveenergy/openbyte/internal/websocket"
)

func TestServerAllowedOriginWildcard(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"*.example.com"})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, testStreamID)
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, testOriginWildcard); err != nil {
		t.Fatalf("expected wildcard origin to be allowed: %v", err)
	}
}

func TestServerAllowedOriginHostMatch(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"foo.example.com"})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, testStreamID)
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, testOriginHostWithPort); err != nil {
		t.Fatalf("expected host-only origin to be allowed: %v", err)
	}
}

func TestServerSetAllowedOriginsTrimsWhitespace(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"  https://foo.example.com  "})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, testStreamID)
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, testOriginWildcard); err != nil {
		t.Fatalf("expected trimmed origin to match: %v", err)
	}
}

func TestWebSocketEmptyOriginWithConfiguredOrigins(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{testOriginWildcard})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, testStreamID)
	}))
	t.Cleanup(testServer.Close)

	if dialWebSocket(t, testServer.URL, "") == nil {
		t.Fatal("expected empty origin to be rejected when explicit allow-list is configured")
	}
}
