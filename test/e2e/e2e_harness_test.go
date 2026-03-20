package e2e

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

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
