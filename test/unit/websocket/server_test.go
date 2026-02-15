package websocket_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	obytewebsocket "github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

func dialWebSocket(t *testing.T, serverURL string, origin string) error {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return err
	}
	parsed.Scheme = "ws"

	headers := http.Header{}
	if origin != "" {
		headers.Set("Origin", origin)
	}

	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), headers)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func dialWebSocketConn(t *testing.T, serverURL string, origin string) (*gorilla.Conn, error) {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	parsed.Scheme = "ws"
	headers := http.Header{}
	if origin != "" {
		headers.Set("Origin", origin)
	}
	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), headers)
	if err != nil {
		return nil, err
	}
	return conn, nil
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

func TestWebSocketEmptyOriginWithConfiguredOrigins(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetAllowedOrigins([]string{"https://foo.example.com"})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "test")
	}))
	t.Cleanup(testServer.Close)

	if err := dialWebSocket(t, testServer.URL, ""); err == nil {
		t.Fatal("expected empty origin to be rejected when explicit allow-list is configured")
	}
}

func TestServerCloseClosesActiveConnections(t *testing.T) {
	server := obytewebsocket.NewServer()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "stream-close")
	}))
	defer testServer.Close()

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	parsed.Scheme = "ws"
	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	server.Close()
	server.Close()

	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected connection to close after server.Close")
	}
}

func TestBroadcastMetricsSendsTerminalStatusOnce(t *testing.T) {
	server := obytewebsocket.NewServer()
	t.Cleanup(server.Close)

	const streamID = "stream-terminal-once"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, streamID)
	}))
	defer testServer.Close()

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	parsed.Scheme = "ws"
	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	snapshot := types.StreamSnapshot{
		Status:    types.StreamStatusCompleted,
		StartTime: time.Now().Add(-1 * time.Second),
		Config: types.StreamConfig{
			Protocol:  types.ProtocolTCP,
			Direction: types.DirectionDownload,
			Duration:  1 * time.Second,
			Streams:   1,
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.BroadcastMetrics(streamID, snapshot)
		}()
	}
	wg.Wait()

	completeCount := 0
	for {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Millisecond))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal websocket message: %v", err)
		}
		if msg["type"] == "complete" {
			completeCount++
		}
	}

	if completeCount != 1 {
		t.Fatalf("complete message count = %d, want 1", completeCount)
	}
}

func TestConcurrentBroadcastRemoval(t *testing.T) {
	server := obytewebsocket.NewServer()
	t.Cleanup(server.Close)

	const streamID = "stream-concurrent-removal"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, streamID)
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	snapshot := types.StreamSnapshot{
		Status:    types.StreamStatusRunning,
		StartTime: time.Now(),
		Config: types.StreamConfig{
			Protocol:  types.ProtocolTCP,
			Direction: types.DirectionDownload,
			Duration:  2 * time.Second,
			Streams:   1,
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			server.BroadcastMetrics(streamID, snapshot)
		}
	}()

	time.Sleep(20 * time.Millisecond)
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast loop did not complete during concurrent client removal")
	}
}
