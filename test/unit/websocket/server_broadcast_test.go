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
