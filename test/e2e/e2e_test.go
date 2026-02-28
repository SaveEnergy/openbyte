package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
	ws "github.com/saveenergy/openbyte/internal/websocket"
)

type TestServer struct {
	server       *http.Server
	listener     net.Listener
	manager      *stream.Manager
	wsServer     *ws.Server
	streamServer *stream.Server
	resultsStore *results.Store
	baseURL      string
	tcpTestAddr  string
	udpTestAddr  string
}

const (
	jsonContentType          = "application/json"
	noStoreValue             = "no-store"
	loopbackIP               = "127.0.0.1"
	openbyteJSPath           = "/openbyte.js"
	marshalReqErr            = "Failed to marshal request: %v"
	startTestErr             = "Failed to start test: %v"
	decodeStartErr           = "Failed to decode start response: %v"
	decodeRespErr            = "Failed to decode response: %v"
	streamStartAPI           = "/api/v1/stream/start"
	httpPrefix               = "http://"
	httpsPrefix              = "https://"
	wsPrefix                 = "ws://"
	wssPrefix                = "wss://"
	protocolKey              = "protocol"
	directionKey             = "direction"
	durationKey              = "duration"
	streamsKey               = "streams"
	protocolTCP              = "tcp"
	directionDL              = "download"
	streamIDKey              = "stream_id"
	websocketURLKey          = "websocket_url"
	wsURLMissingErr          = "websocket_url not found in response"
	streamIDMissingErr       = "stream_id not found in response"
	streamIDEmptyErr         = "stream_id is empty"
	respMissingStreamID      = "Response missing stream_id"
	respMissingWSURLErr      = "Response missing websocket_url"
	wsMessageStreamIDFmt     = "WebSocket message stream_id = %s, want %s"
	wsConnectErrFmt          = "WebSocket connection failed: %v"
	wsDialErrFmt             = "WebSocket failed: %v"
	statusCheckErrFmt        = "Status check failed: %v"
	statusCheckCodeFmt       = "Status check failed: %d"
	wsErrorLogFmt            = "WebSocket error: %v"
	statusCreated            = http.StatusCreated
	statusOK                 = http.StatusOK
	wsReadErrFmt             = "Failed to read WebSocket message: %v"
	createStreamServerErrFmt = "Failed to create stream server: %v"
	createResultsStoreErrFmt = "Failed to create results store: %v"
	absPathErrFmt            = "Failed to get absolute path: %v"
	createListenerErrFmt     = "Failed to create listener: %v"
	reserveTCPPortErrFmt     = "reserve tcp port: %v"
	reserveUDPPortErrFmt     = "reserve udp port: %v"
	missingTestAddrsErr      = "missing stream test addresses"
	apiRequestErrFmt         = "API request failed: %v"
	apiStatusBodyFmt         = "API status = %d, want %d. Body: %s"
	wsMissingTypeErr         = "WebSocket message missing type"
	expectedWSRejectErr      = "expected websocket origin rejection"
	unexpectedStatusFmt      = "unexpected status code = %d, want %d"
	startTestFailedFmt       = "Start test failed: %d. Body: %s"
	streamIDInvalidRespFmt   = "stream_id missing or invalid in response: %#v"
	wsURLInvalidRespFmt      = "websocket_url missing or invalid in response: %#v"
	msgMissingTypeErr        = "Message missing type"
	noWSMessagesErr          = "No WebSocket messages received"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping heavy e2e test in short mode")
	}
}

func NewTestServer(t *testing.T) *TestServer {
	return NewTestServerWithOrigins(t, nil)
}

func NewTestServerWithOrigins(t *testing.T, allowedOrigins []string) *TestServer {
	cfg := config.DefaultConfig()
	cfg.BindAddress = loopbackIP
	cfg.Port = "0"
	cfg.TCPTestPort = reserveTCPPort(t)
	cfg.UDPTestPort = reserveUDPPort(t)

	streamServer, err := stream.NewServer(cfg)
	if err != nil {
		t.Fatalf(createStreamServerErrFmt, err)
	}

	manager := stream.NewManager(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)
	manager.Start()

	handler := api.NewHandler(manager)
	handler.SetConfig(cfg)
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)
	resultsStore, err := results.New(t.TempDir()+"/results.db", 1000)
	if err != nil {
		_ = streamServer.Close()
		manager.Stop()
		t.Fatalf(createResultsStoreErrFmt, err)
	}
	router.SetResultsHandler(results.NewHandler(resultsStore))

	wsServer := ws.NewServer()
	wsServer.SetAllowedOrigins(allowedOrigins)
	router.SetWebSocketHandler(wsServer.HandleStream)

	webDir := "./web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "../../web"
	}
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		t.Fatalf(absPathErrFmt, err)
	}
	router.SetWebRoot(absWebDir)

	httpHandler := router.SetupRoutes()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf(createListenerErrFmt, err)
	}

	srv := &http.Server{
		Handler: httpHandler,
	}

	go srv.Serve(listener)

	time.Sleep(100 * time.Millisecond)

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://localhost:%d", port)

	return &TestServer{
		server:       srv,
		listener:     listener,
		manager:      manager,
		wsServer:     wsServer,
		streamServer: streamServer,
		resultsStore: resultsStore,
		baseURL:      baseURL,
		tcpTestAddr:  net.JoinHostPort(loopbackIP, fmt.Sprintf("%d", cfg.TCPTestPort)),
		udpTestAddr:  net.JoinHostPort(loopbackIP, fmt.Sprintf("%d", cfg.UDPTestPort)),
	}
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", loopbackIP+":0")
	if err != nil {
		t.Fatalf(reserveTCPPortErrFmt, err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func reserveUDPPort(t *testing.T) int {
	t.Helper()
	addr := &net.UDPAddr{IP: net.ParseIP(loopbackIP), Port: 0}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf(reserveUDPPortErrFmt, err)
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port
}

func (ts *TestServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts.server.Shutdown(ctx)
	ts.listener.Close()
	ts.manager.Stop()
	if ts.resultsStore != nil {
		ts.resultsStore.Close()
	}
	if ts.streamServer != nil {
		ts.streamServer.Close()
	}
}

func getStreamTestAddrs(t *testing.T, ts *TestServer) (string, string) {
	t.Helper()
	if ts.tcpTestAddr == "" || ts.udpTestAddr == "" {
		t.Fatal(missingTestAddrsErr)
	}
	return ts.tcpTestAddr, ts.udpTestAddr
}

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
