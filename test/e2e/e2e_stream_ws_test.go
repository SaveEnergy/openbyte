package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestAPIStartTest(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		protocolKey:   protocolTCP,
		directionKey:  directionDL,
		durationKey:   5,
		streamsKey:    2,
		"packet_size": 1400,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+streamStartAPI, jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf(apiRequestErrFmt, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != statusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf(apiStatusBodyFmt, resp.StatusCode, statusCreated, string(bodyBytes))
		return
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf(decodeRespErr, err)
	}

	if data[streamIDKey] == nil {
		t.Error(respMissingStreamID)
	}
	if data[websocketURLKey] == nil {
		t.Error(respMissingWSURLErr)
	}
}

func TestWebSocketConnection(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		protocolKey:  protocolTCP,
		directionKey: directionDL,
		durationKey:  3,
		streamsKey:   1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+streamStartAPI, jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf(startTestErr, err)
	}
	defer resp.Body.Close()

	var startResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf(decodeStartErr, err)
	}

	streamID, ok := startResp[streamIDKey].(string)
	if !ok {
		t.Fatal(streamIDMissingErr)
	}

	if streamID == "" {
		t.Fatal(streamIDEmptyErr)
	}

	wsURL, ok := startResp[websocketURLKey].(string)
	if !ok {
		t.Fatal(wsURLMissingErr)
	}

	if strings.HasPrefix(wsURL, "/") {
		wsURL = strings.Replace(ts.baseURL, httpPrefix, wsPrefix, 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, httpPrefix, wsPrefix, 1)
		wsURL = strings.Replace(wsURL, httpsPrefix, wssPrefix, 1)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf(wsConnectErrFmt, err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf(wsReadErrFmt, err)
	}

	if msg["type"] == nil {
		t.Error(wsMissingTypeErr)
	}

	if msgStreamID, ok := msg["stream_id"].(string); ok && msgStreamID != streamID {
		t.Errorf(wsMessageStreamIDFmt, msgStreamID, streamID)
	}
}

func TestWebSocketOriginRejected(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServerWithOrigins(t, []string{"https://allowed.example"})
	defer ts.Close()

	reqBody := map[string]any{
		protocolKey:  protocolTCP,
		directionKey: directionDL,
		durationKey:  3,
		streamsKey:   1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+streamStartAPI, jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf(startTestErr, err)
	}
	defer resp.Body.Close()

	var startResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf(decodeStartErr, err)
	}

	wsURL, ok := startResp[websocketURLKey].(string)
	if !ok {
		t.Fatal(wsURLMissingErr)
	}

	if strings.HasPrefix(wsURL, "/") {
		wsURL = strings.Replace(ts.baseURL, httpPrefix, wsPrefix, 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, httpPrefix, wsPrefix, 1)
		wsURL = strings.Replace(wsURL, httpsPrefix, wssPrefix, 1)
	}

	headers := http.Header{}
	headers.Set("Origin", "https://evil.example")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatal(expectedWSRejectErr)
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Fatalf(unexpectedStatusFmt, resp.StatusCode, http.StatusForbidden)
	}
}

func resolveWebSocketURL(baseURL, wsURL string) string {
	if strings.HasPrefix(wsURL, "/") {
		return strings.Replace(baseURL, httpPrefix, wsPrefix, 1) + wsURL
	}
	wsURL = strings.Replace(wsURL, httpPrefix, wsPrefix, 1)
	wsURL = strings.Replace(wsURL, httpsPrefix, wssPrefix, 1)
	return wsURL
}

func readWebSocketMessages(t *testing.T, conn *websocket.Conn, timeout time.Duration) int {
	t.Helper()
	messageCount := 0
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)
	for time.Now().Before(deadline) {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				t.Logf(wsErrorLogFmt, err)
			}
			break
		}
		messageCount++
		if msg["type"] == nil {
			t.Error(msgMissingTypeErr)
		}
	}
	return messageCount
}

func TestFullFlow(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		protocolKey:  protocolTCP,
		directionKey: directionDL,
		durationKey:  5,
		streamsKey:   2,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+streamStartAPI, jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf(startTestErr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != statusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf(startTestFailedFmt, resp.StatusCode, string(bodyBytes))
	}

	var startResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf(decodeRespErr, err)
	}

	streamID, ok := startResp[streamIDKey].(string)
	if !ok || streamID == "" {
		t.Fatalf(streamIDInvalidRespFmt, startResp[streamIDKey])
	}

	statusResp, err := http.Get(ts.baseURL + "/api/v1/stream/" + streamID + "/status")
	if err != nil {
		t.Fatalf(statusCheckErrFmt, err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != statusOK {
		t.Errorf(statusCheckCodeFmt, statusResp.StatusCode)
	}

	wsURL, ok := startResp[websocketURLKey].(string)
	if !ok || wsURL == "" {
		t.Fatalf(wsURLInvalidRespFmt, startResp[websocketURLKey])
	}
	wsURL = resolveWebSocketURL(ts.baseURL, wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf(wsDialErrFmt, err)
	}
	defer conn.Close()

	messageCount := readWebSocketMessages(t, conn, 3*time.Second)
	if messageCount == 0 {
		t.Error(noWSMessagesErr)
	}
}
