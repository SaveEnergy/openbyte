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
)

type TestServer struct {
	server       *http.Server
	listener     net.Listener
	resultsStore *results.Store
	baseURL      string
}

const (
	jsonContentType          = "application/json"
	openbyteJSPath           = "/openbyte.js"
	noStoreValue             = "no-store"
	absPathErrFmt            = "Failed to get absolute path: %v"
	createListenerErrFmt     = "Failed to create listener: %v"
	createResultsStoreErrFmt = "Failed to create results store: %v"
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
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = "0"

	handler := api.NewHandler()
	handler.SetConfig(cfg)
	router := api.NewRouter(handler, cfg)
	router.SetRateLimiter(cfg)
	router.SetAllowedOrigins(allowedOrigins)

	resultsStore, err := results.New(t.TempDir()+"/results.db", 1000)
	if err != nil {
		t.Fatalf(createResultsStoreErrFmt, err)
	}
	router.SetResultsHandler(results.NewHandler(resultsStore))

	webDir := "./web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "../../web"
	}
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		t.Fatalf(absPathErrFmt, err)
	}
	router.SetWebRoot(absWebDir)

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf(createListenerErrFmt, err)
	}

	srv := &http.Server{Handler: router.SetupRoutes()}
	go srv.Serve(listener)

	time.Sleep(100 * time.Millisecond)

	port := listener.Addr().(*net.TCPAddr).Port
	return &TestServer{
		server:       srv,
		listener:     listener,
		resultsStore: resultsStore,
		baseURL:      fmt.Sprintf("http://localhost:%d", port),
	}
}

func (ts *TestServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ts.server.Shutdown(ctx)
	_ = ts.listener.Close()
	if ts.resultsStore != nil {
		ts.resultsStore.Close()
	}
}
