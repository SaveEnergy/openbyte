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
	cfg.BindAddress = "127.0.0.1"
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
		tcpTestAddr:  net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.TCPTestPort)),
		udpTestAddr:  net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", cfg.UDPTestPort)),
	}
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func reserveUDPPort(t *testing.T) int {
	t.Helper()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
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

func TestHealthEndpoint(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var data map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("Health status = %s, want 'ok'", data["status"])
	}
}

func TestPingEndpoint(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/api/v1/ping")
	if err != nil {
		t.Fatalf("Ping request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ping status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode ping response: %v", err)
	}
	if pong, ok := data["pong"].(bool); !ok || !pong {
		t.Fatalf("pong = %v, want true", data["pong"])
	}
	if _, ok := data["timestamp"].(float64); !ok {
		t.Fatalf("timestamp missing or invalid type: %T", data["timestamp"])
	}
}

func getStreamTestAddrs(t *testing.T, ts *TestServer) (string, string) {
	t.Helper()
	if ts.tcpTestAddr == "" || ts.udpTestAddr == "" {
		t.Fatal("missing stream test addresses")
	}
	return ts.tcpTestAddr, ts.udpTestAddr
}

func TestStreamServerTCPDownload(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	tcpAddr, _ := getStreamTestAddrs(t, ts)
	conn, err := net.DialTimeout("tcp", tcpAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{'D'}); err != nil {
		t.Fatalf("write tcp command: %v", err)
	}

	buf := make([]byte, 2048)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read tcp payload: %v", err)
	}
	if n <= 0 {
		t.Fatalf("read bytes = %d, want > 0", n)
	}
}

func TestStreamServerUDPDownload(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	_, udpAddr := getStreamTestAddrs(t, ts)
	serverAddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		t.Fatalf("resolve udp addr: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{'D'}); err != nil {
		t.Fatalf("write udp command: %v", err)
	}

	buf := make([]byte, 2048)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set udp read deadline: %v", err)
	}
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read udp payload: %v", err)
	}
	if n <= 0 {
		t.Fatalf("read udp bytes = %d, want > 0", n)
	}

	_, _ = conn.Write([]byte{'S'})
}

func TestResultsSaveAndGet(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	saveReq := map[string]interface{}{
		"download_mbps":      123.45,
		"upload_mbps":        67.89,
		"latency_ms":         12.3,
		"jitter_ms":          1.2,
		"loaded_latency_ms":  18.4,
		"bufferbloat_grade":  "A",
		"ipv4":               "192.0.2.1",
		"ipv6":               "",
		"server_name":        "e2e-server",
	}
	body, err := json.Marshal(saveReq)
	if err != nil {
		t.Fatalf("marshal save request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/results", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("save result request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("save result status = %d, want %d, body=%s", resp.StatusCode, http.StatusCreated, string(data))
	}

	var saveResp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&saveResp); err != nil {
		t.Fatalf("decode save response: %v", err)
	}
	if saveResp.ID == "" || saveResp.URL == "" {
		t.Fatalf("save response missing id/url: %#v", saveResp)
	}

	getResp, err := http.Get(ts.baseURL + "/api/v1/results/" + saveResp.ID)
	if err != nil {
		t.Fatalf("get result request failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(getResp.Body)
		t.Fatalf("get result status = %d, want %d, body=%s", getResp.StatusCode, http.StatusOK, string(data))
	}
	if got := getResp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("get result cache-control = %q, want %q", got, "no-store")
	}

	var saved map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&saved); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if id, ok := saved["id"].(string); !ok || id != saveResp.ID {
		t.Fatalf("saved id = %#v, want %q", saved["id"], saveResp.ID)
	}
}

func TestFrontendLoads(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/")
	if err != nil {
		t.Fatalf("Frontend request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Frontend status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read frontend: %v", err)
	}

	html := string(body)

	checks := []string{
		"<title>openByte",
		"openByte",
		"app.js",
		"style.css",
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("Frontend missing: %s", check)
		}
	}
}

func TestStaticFiles(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	files := []string{
		"/app.js",
		"/style.css",
	}

	for _, file := range files {
		resp, err := http.Get(ts.baseURL + file)
		if err != nil {
			t.Errorf("Failed to load %s: %v", file, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
				t.Errorf("failed to drain %s response body: %v", file, drainErr)
			}
			t.Errorf("%s status = %d, want %d", file, resp.StatusCode, http.StatusOK)
		}

		contentType := resp.Header.Get("Content-Type")
		if file == "/app.js" && !strings.Contains(contentType, "javascript") {
			t.Errorf("%s content-type = %s, want javascript", file, contentType)
		}
		if file == "/style.css" && !strings.Contains(contentType, "css") {
			t.Errorf("%s content-type = %s, want css", file, contentType)
		}
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close %s response body: %v", file, err)
		}
	}
}

func TestJavaScriptFunctions(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.baseURL + "/app.js")
	if err != nil {
		t.Fatalf("Failed to load app.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("app.js status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read app.js: %v", err)
	}

	js := string(body)

	requiredFunctions := []string{
		"function startTest",
		"function runTest",
		"function updateSpeed",
		"function showResults",
		"function resetToIdle",
		"function showError",
	}

	for _, fn := range requiredFunctions {
		if !strings.Contains(js, fn) {
			t.Errorf("Missing required function: %s", fn)
		}
	}

	requiredVars := []string{
		"apiBase",
		"const state",
		"const elements",
	}

	for _, v := range requiredVars {
		if !strings.Contains(js, v) {
			t.Errorf("Missing required variable: %s", v)
		}
	}

	openBraces := strings.Count(js, "{")
	closeBraces := strings.Count(js, "}")
	if openBraces != closeBraces {
		t.Errorf("Unbalanced braces: %d open, %d close", openBraces, closeBraces)
	}
}

func TestAPIStartTest(t *testing.T) {
	skipIfShort(t)
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"protocol":    "tcp",
		"direction":   "download",
		"duration":    5,
		"streams":     2,
		"packet_size": 1400,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("API status = %d, want %d. Body: %s", resp.StatusCode, http.StatusCreated, string(bodyBytes))
		return
	}

	var data map[string]interface{}
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

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to start test: %v", err)
	}
	defer resp.Body.Close()

	var startResp map[string]interface{}
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
		wsURL = strings.Replace(ts.baseURL, "http://", "ws://", 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var msg map[string]interface{}
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

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to start test: %v", err)
	}
	defer resp.Body.Close()

	var startResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		t.Fatalf("Failed to decode start response: %v", err)
	}

	wsURL, ok := startResp["websocket_url"].(string)
	if !ok {
		t.Fatal("websocket_url not found in response")
	}

	if strings.HasPrefix(wsURL, "/") {
		wsURL = strings.Replace(ts.baseURL, "http://", "ws://", 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
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

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  5,
		"streams":   2,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to start test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Start test failed: %d. Body: %s", resp.StatusCode, string(bodyBytes))
	}

	var startResp map[string]interface{}
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
		wsURL = strings.Replace(ts.baseURL, "http://", "ws://", 1) + wsURL
	} else {
		wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
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
		var msg map[string]interface{}
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
