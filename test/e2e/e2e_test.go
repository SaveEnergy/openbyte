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
	jsonContentType = "application/json"
	noStoreValue    = "no-store"
	loopbackIP      = "127.0.0.1"
	openbyteJSPath  = "/openbyte.js"
	marshalReqErr   = "Failed to marshal request: %v"
	startTestErr    = "Failed to start test: %v"
	streamStartAPI  = "/api/v1/stream/start"
	httpPrefix      = "http://"
	httpsPrefix     = "https://"
	wsPrefix        = "ws://"
	wssPrefix       = "wss://"
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
		t.Fatalf("Failed to create stream server: %v", err)
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
		t.Fatalf("Failed to create results store: %v", err)
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
		t.Fatalf("Failed to get absolute path: %v", err)
	}
	router.SetWebRoot(absWebDir)

	httpHandler := router.SetupRoutes()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
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
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func reserveUDPPort(t *testing.T) int {
	t.Helper()
	addr := &net.UDPAddr{IP: net.ParseIP(loopbackIP), Port: 0}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("reserve udp port: %v", err)
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
		t.Fatal("missing stream test addresses")
	}
	return ts.tcpTestAddr, ts.udpTestAddr
}

func TestAPIStartTest(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		"protocol":    "tcp",
		"direction":   "download",
		"duration":    5,
		"streams":     2,
		"packet_size": 1400,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+streamStartAPI, jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("API status = %d, want %d. Body: %s", resp.StatusCode, http.StatusCreated, string(bodyBytes))
		return
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if data["stream_id"] == nil {
		t.Error("Response missing stream_id")
	}
	if data["websocket_url"] == nil {
		t.Error("Response missing websocket_url")
	}
}

func TestWebSocketConnection(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf(marshalReqErr, err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", jsonContentType, bytes.NewReader(body))
	if err != nil {
		t.Fatalf(startTestErr, err)
	}
	defer resp.Body.Close()

	var startResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf("Failed to decode start response: %v", err)
	}

	streamID, ok := startResp["stream_id"].(string)
	if !ok {
		t.Fatal("stream_id not found in response")
	}

	if streamID == "" {
		t.Fatal("stream_id is empty")
	}

	wsURL, ok := startResp["websocket_url"].(string)
	if !ok {
		t.Fatal("websocket_url not found in response")
	}

	if strings.HasPrefix(wsURL, "/") {
		wsURL = strings.Replace(ts.baseURL, httpPrefix, wsPrefix, 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, httpPrefix, wsPrefix, 1)
		wsURL = strings.Replace(wsURL, httpsPrefix, wssPrefix, 1)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to read WebSocket message: %v", err)
	}

	if msg["type"] == nil {
		t.Error("WebSocket message missing type")
	}

	if msgStreamID, ok := msg["stream_id"].(string); ok && msgStreamID != streamID {
		t.Errorf("WebSocket message stream_id = %s, want %s", msgStreamID, streamID)
	}
}

func TestWebSocketOriginRejected(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServerWithOrigins(t, []string{"https://allowed.example"})
	defer ts.Close()

	reqBody := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
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
		t.Fatalf("Failed to decode start response: %v", err)
	}

	wsURL, ok := startResp["websocket_url"].(string)
	if !ok {
		t.Fatal("websocket_url not found in response")
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
		t.Fatal("expected websocket origin rejection")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestFullFlow(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]any{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  5,
		"streams":   2,
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

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Start test failed: %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	var startResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	streamID, ok := startResp["stream_id"].(string)
	if !ok || streamID == "" {
		t.Fatalf("stream_id missing or invalid in response: %#v", startResp["stream_id"])
	}

	statusResp, err := http.Get(ts.baseURL + "/api/v1/stream/" + streamID + "/status")
	if err != nil {
		t.Fatalf("Status check failed: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("Status check failed: %d", statusResp.StatusCode)
	}

	wsURL, ok := startResp["websocket_url"].(string)
	if !ok || wsURL == "" {
		t.Fatalf("websocket_url missing or invalid in response: %#v", startResp["websocket_url"])
	}
	if strings.HasPrefix(wsURL, "/") {
		wsURL = strings.Replace(ts.baseURL, httpPrefix, wsPrefix, 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, httpPrefix, wsPrefix, 1)
		wsURL = strings.Replace(wsURL, httpsPrefix, wssPrefix, 1)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket failed: %v", err)
	}
	defer conn.Close()

	messageCount := 0
	deadline := time.Now().Add(3 * time.Second)
	conn.SetReadDeadline(deadline)

	for time.Now().Before(deadline) {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				t.Logf("WebSocket error: %v", err)
			}
			break
		}
		messageCount++

		if msg["type"] == nil {
			t.Error("Message missing type")
		}
	}

	if messageCount == 0 {
		t.Error("No WebSocket messages received")
	}
}
