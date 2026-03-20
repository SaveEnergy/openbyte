package websocket_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
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
