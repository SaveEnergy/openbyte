package e2e

import (
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

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
	ws "github.com/saveenergy/openbyte/internal/websocket"
)

type TestServer struct {
	server       *http.Server
	listener     net.Listener
	manager      *stream.Manager
	wsServer     *ws.Server
	streamServer *stream.Server
	baseURL      string
}

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func NewTestServer(t *testing.T) *TestServer {
	return NewTestServerWithOrigins(t, nil)
}

func NewTestServerWithOrigins(t *testing.T, allowedOrigins []string) *TestServer {
	cfg := config.DefaultConfig()
	cfg.Port = "0"
	cfg.TCPTestPort = 0
	cfg.UDPTestPort = 0

	streamServer, err := stream.NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create stream server: %v", err)
	}

	manager := stream.NewManager(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)
	manager.Start()

	handler := api.NewHandler(manager)
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)

	wsServer := ws.NewServer()
	wsServer.SetAllowedOrigins(allowedOrigins)

	muxRouter := mux.NewRouter()

	v1 := muxRouter.PathPrefix("/api/v1").Subrouter()
	if limiter := router.GetLimiter(); limiter != nil {
		v1.Use(api.RateLimitMiddleware(limiter))
	}
	v1.HandleFunc("/stream/start", handler.StartStream).Methods("POST")
	v1.HandleFunc("/stream/{id}/status", router.HandleWithID(handler.GetStreamStatus)).Methods("GET")
	v1.HandleFunc("/stream/{id}/results", router.HandleWithID(handler.GetStreamResults)).Methods("GET")
	v1.HandleFunc("/stream/{id}/cancel", router.HandleWithID(handler.CancelStream)).Methods("POST")

	muxRouter.HandleFunc("/api/v1/stream/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		streamID := vars["id"]
		if streamID == "" {
			http.Error(w, "stream ID required", http.StatusBadRequest)
			return
		}
		wsServer.HandleStream(w, r, streamID)
	})

	muxRouter.HandleFunc("/health", router.HealthCheck).Methods("GET")

	webDir := "./web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "../../web"
	}
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	muxRouter.PathPrefix("/").Handler(http.FileServer(http.Dir(absWebDir)))

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	srv := &http.Server{
		Handler: muxRouter,
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
		baseURL:      baseURL,
	}
}

func (ts *TestServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ts.server.Shutdown(ctx)
	ts.listener.Close()
	ts.manager.Stop()
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

func TestFrontendLoads(t *testing.T) {
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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want %d", file, resp.StatusCode, http.StatusOK)
		}

		contentType := resp.Header.Get("Content-Type")
		if file == "/app.js" && !strings.Contains(contentType, "javascript") {
			t.Errorf("%s content-type = %s, want javascript", file, contentType)
		}
		if file == "/style.css" && !strings.Contains(contentType, "css") {
			t.Errorf("%s content-type = %s, want css", file, contentType)
		}
	}
}

func TestJavaScriptFunctions(t *testing.T) {
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
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"protocol":    "tcp",
		"direction":   "download",
		"duration":    5,
		"streams":     2,
		"packet_size": 1500,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", strings.NewReader(string(body)))
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
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", strings.NewReader(string(body)))
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
	ts := NewTestServerWithOrigins(t, []string{"https://allowed.example"})
	defer ts.Close()

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  3,
		"streams":   1,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", strings.NewReader(string(body)))
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
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := map[string]interface{}{
		"protocol":  "tcp",
		"direction": "download",
		"duration":  5,
		"streams":   2,
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.baseURL+"/api/v1/stream/start", "application/json", strings.NewReader(string(body)))
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

	streamID := startResp["stream_id"].(string)

	statusResp, err := http.Get(ts.baseURL + "/api/v1/stream/" + streamID + "/status")
	if err != nil {
		t.Fatalf("Status check failed: %v", err)
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		t.Errorf("Status check failed: %d", statusResp.StatusCode)
	}

	wsURL := startResp["websocket_url"].(string)
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
