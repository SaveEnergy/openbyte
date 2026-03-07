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

const (
	testStreamID              = "test"
	testOriginWildcard        = "https://foo.example.com"
	testOriginHostWithPort    = "https://foo.example.com:8443"
	streamIDTerminalOnce      = "stream-terminal-once"
	streamIDTerminalClose     = "stream-terminal-close"
	streamIDConcurrentRemoval = "stream-concurrent-removal"
	messageTypeComplete       = "complete"
	wsConnectedReadTimeout    = 2 * time.Second
	wsTerminalReadTimeout     = 1 * time.Second
	wsDrainReadTimeout        = 120 * time.Millisecond
	parseTestServerURLFmt     = "parse test server URL: %v"
	dialWebsocketFmt          = "dial websocket: %v"
	readConnectedMsgFmt       = "read connected message: %v"
	unmarshalWsMsgFmt         = "unmarshal websocket message: %v"
	completeMsgCountWantFmt   = "complete message count = %d, want 1"
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

func TestServerCloseClosesActiveConnections(t *testing.T) {
	server := obytewebsocket.NewServer()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "stream-close")
	}))
	defer testServer.Close()

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf(parseTestServerURLFmt, err)
	}
	parsed.Scheme = "ws"
	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
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

	const streamID = streamIDTerminalOnce
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, streamID)
	}))
	defer testServer.Close()

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf(parseTestServerURLFmt, err)
	}
	parsed.Scheme = "ws"
	conn, _, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
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
	for range 12 {
		wg.Go(func() {
			server.BroadcastMetrics(streamID, snapshot)
		})
	}
	wg.Wait()

	completeCount := 0
	for {
		_ = conn.SetReadDeadline(time.Now().Add(wsDrainReadTimeout))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf(unmarshalWsMsgFmt, err)
		}
		if msg["type"] == messageTypeComplete {
			completeCount++
		}
	}

	if completeCount != 1 {
		t.Fatalf(completeMsgCountWantFmt, completeCount)
	}
}

func TestTerminalBroadcastClosesConnection(t *testing.T) {
	server := obytewebsocket.NewServer()
	t.Cleanup(server.Close)

	const streamID = streamIDTerminalClose
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, streamID)
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
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
	server.BroadcastMetrics(streamID, snapshot)

	completeCount := 0
	for {
		_ = conn.SetReadDeadline(time.Now().Add(wsTerminalReadTimeout))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf(unmarshalWsMsgFmt, err)
		}
		if msg["type"] == messageTypeComplete {
			completeCount++
		}
	}
	if completeCount != 1 {
		t.Fatalf(completeMsgCountWantFmt, completeCount)
	}
}

func TestConcurrentBroadcastRemoval(t *testing.T) {
	server := obytewebsocket.NewServer()
	t.Cleanup(server.Close)

	const streamID = streamIDConcurrentRemoval
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, streamID)
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
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
		for range 200 {
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

func TestServerRejectsWhenGlobalConnectionLimitExceeded(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetConnectionLimits(1, 0)
	t.Cleanup(server.Close)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "stream-global-limit")
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
	}

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf(parseTestServerURLFmt, err)
	}
	parsed.Scheme = "ws"
	secondConn, resp, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if secondConn != nil {
		secondConn.Close()
	}
	if err == nil {
		t.Fatal("expected second websocket dial to fail under global cap")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %v, want %d", responseStatusCode(resp), http.StatusServiceUnavailable)
	}
}

func TestServerRejectsWhenPerIPConnectionLimitExceeded(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetConnectionLimits(2, 1)
	t.Cleanup(server.Close)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "stream-per-ip-limit")
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
	}

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf(parseTestServerURLFmt, err)
	}
	parsed.Scheme = "ws"
	secondConn, resp, err := gorilla.DefaultDialer.Dial(parsed.String(), nil)
	if secondConn != nil {
		secondConn.Close()
	}
	if err == nil {
		t.Fatal("expected second websocket dial to fail under per-IP cap")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %v, want %d", responseStatusCode(resp), http.StatusServiceUnavailable)
	}
}

func TestServerReleasesConnectionSlotAfterDisconnect(t *testing.T) {
	server := obytewebsocket.NewServer()
	server.SetConnectionLimits(1, 1)
	t.Cleanup(server.Close)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleStream(w, r, "stream-release")
	}))
	defer testServer.Close()

	conn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf(dialWebsocketFmt, err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
	}
	_ = conn.Close()
	time.Sleep(100 * time.Millisecond)

	secondConn, err := dialWebSocketConn(t, testServer.URL, "")
	if err != nil {
		t.Fatalf("dial websocket after release: %v", err)
	}
	defer secondConn.Close()
	_ = secondConn.SetReadDeadline(time.Now().Add(wsConnectedReadTimeout))
	if _, _, err := secondConn.ReadMessage(); err != nil {
		t.Fatalf(readConnectedMsgFmt, err)
	}
}

func responseStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
