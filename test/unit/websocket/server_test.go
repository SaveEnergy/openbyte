package websocket_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gorilla "github.com/gorilla/websocket"
	obytewebsocket "github.com/saveenergy/openbyte/internal/websocket"
)

func dialWebSocket(t *testing.T, serverURL string, origin string) error {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	parsed.Scheme = "ws"

	headers := http.Header{}
	headers.Set("Origin", origin)

	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), headers)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func TestServerAllowedOriginWildcard(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"*.example.com"})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "test")
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, "https://foo.example.com"); err != nil {
		t.Fatalf("expected wildcard origin to be allowed: %v", err)
	}
}

func TestServerAllowedOriginHostMatch(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"foo.example.com"})

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "test")
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, "https://foo.example.com:8443"); err != nil {
		t.Fatalf("expected host-only origin to be allowed: %v", err)
	}
}
