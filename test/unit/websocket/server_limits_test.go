package websocket_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	obytewebsocket "github.com/saveenergy/openbyte/internal/websocket"
)

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
