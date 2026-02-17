package server

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/registry"
	"github.com/saveenergy/openbyte/internal/results"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/internal/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

var (
	exitSuccess = 0
	exitFailure = 1
)

type serverFlagValues struct {
	port               *string
	bindAddress        *string
	tcpTestPort        *int
	udpTestPort        *int
	serverID           *string
	serverName         *string
	serverLocation     *string
	serverRegion       *string
	publicHost         *string
	capacityGbps       *int
	maxConcurrentTests *int
	maxConcurrentPerIP *int
	maxStreams         *int
	maxTestDuration    *string
	rateLimitPerIP     *int
	globalRateLimit    *int
	allowedOrigins     *string
	trustProxyHeaders  *bool
	trustedProxyCIDRs  *string
	dataDir            *string
	maxStoredResults   *int
	webRoot            *string
	pprofEnabled       *bool
	pprofAddress       *string
	perfStatsInterval  *string
	runtimeMetrics     *bool
	registryEnabled    *bool
	registryMode       *bool
	registryURL        *string
	registryAPIKey     *string
	registryInterval   *string
	registryServerTTL  *string
}

func Run(args []string, version string) int {
	logLevel := logging.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = logging.LevelDebug
	}
	logging.Init(logLevel)

	cfg := config.DefaultConfig()
	if err := cfg.LoadFromEnv(); err != nil {
		logging.Error("Failed to load config", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	fs, fv := buildServerFlagSet(cfg)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		logging.Error("Invalid flags", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	if err := applyServerFlagOverrides(cfg, fs, fv); err != nil {
		logging.Error("Invalid flag values", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	if err := cfg.Validate(); err != nil {
		logging.Error("Invalid configuration", logging.Field{Key: "error", Value: err})
		return exitFailure
	}

	pprofServer := startPprofServer(cfg)
	stopStats := startRuntimeStatsLogger(cfg)
	cleanupOnError := true

	var (
		streamServer    *stream.Server
		manager         *stream.Manager
		resultsStore    *results.Store
		wsServer        *websocket.Server
		registryService *registry.Service
		registryClient  *registry.Client
		err             error
	)
	defer func() {
		if !cleanupOnError {
			return
		}
		if registryClient != nil {
			registryClient.Stop()
		}
		if registryService != nil {
			registryService.Stop()
		}
		if resultsStore != nil {
			resultsStore.Close()
		}
		if manager != nil {
			manager.Stop()
		}
		if wsServer != nil {
			wsServer.Close()
		}
		if streamServer != nil {
			_ = streamServer.Close()
		}
		shutdownPprofServer(pprofServer, 5*time.Second)
		stopStats()
	}()

	streamServer, err = stream.NewServer(cfg)
	if err != nil {
		logging.Error("Failed to start stream server", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	logging.Info("Stream server started",
		logging.Field{Key: "tcp_port", Value: cfg.TCPTestPort},
		logging.Field{Key: "udp_port", Value: cfg.UDPTestPort})

	manager = stream.NewManager(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)
	manager.SetRetentionPeriod(cfg.TestRetentionPeriod)
	manager.SetMetricsUpdateInterval(cfg.MetricsUpdateInterval)
	manager.Start()

	apiHandler := api.NewHandler(manager)
	apiHandler.SetConfig(cfg)
	apiHandler.SetVersion(version)
	wsServer = websocket.NewServer()
	wsServer.SetAllowedOrigins(cfg.AllowedOrigins)
	wsServer.SetPingInterval(cfg.WebSocketPingInterval)

	// Ensure data directory exists for SQLite
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logging.Error("Failed to create data directory", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	resultsStore, err = results.New(cfg.DataDir+"/results.db", cfg.MaxStoredResults)
	if err != nil {
		logging.Error("Failed to open results store", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
	logging.Info("Results store opened",
		logging.Field{Key: "path", Value: cfg.DataDir + "/results.db"},
		logging.Field{Key: "max_results", Value: cfg.MaxStoredResults})

	router := api.NewRouter(apiHandler, cfg)
	router.SetRateLimiter(cfg)
	router.SetClientIPResolver(api.NewClientIPResolver(cfg))
	router.SetAllowedOrigins(cfg.AllowedOrigins)
	router.SetWebSocketHandler(wsServer.HandleStream)
	router.SetResultsHandler(results.NewHandler(resultsStore))
	router.SetWebRoot(cfg.WebRoot)
	if cfg.RuntimeMetrics {
		router.SetRuntimeMetricsHandler(runtimeMetricsHandler())
	}

	var registrars []api.RegistryRegistrar
	if cfg.RegistryMode {
		logging.Info("Starting in registry mode")
		registryService = registry.NewService(cfg.RegistryServerTTL, 30*time.Second)
		registryService.Start()

		registryLogger := logging.NewLogger("registry")
		registryHandler := registry.NewHandler(registryService, registryLogger, cfg.RegistryAPIKey)
		registrars = append(registrars, registryHandler)
	}

	muxRouter := router.SetupRoutes(registrars...)

	var broadcastWg sync.WaitGroup
	broadcastWg.Go(func() {
		broadcastMetrics(manager, wsServer)
	})

	if cfg.RegistryEnabled && !cfg.RegistryMode {
		logger := logging.NewLogger("registry-client")
		registryClient = registry.NewClient(cfg, logger)
		if err := registryClient.Start(manager.ActiveCount); err != nil {
			logging.Warn("Registry client failed to start", logging.Field{Key: "error", Value: err})
		}
	}

	srv := &http.Server{
		Addr:              cfg.BindAddress + ":" + cfg.Port,
		Handler:           muxRouter,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		HTTP2: &http.HTTP2Config{
			StrictMaxConcurrentRequests: true,
		},
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	srvErrCh := make(chan error, 1)
	startHTTPServer(cfg, srv, srvErrCh)
	exitCode := waitForShutdown(quit, srvErrCh)
	shutdownHTTPServer(srv, 30*time.Second)

	shutdownPprofServer(pprofServer, 5*time.Second)
	stopStats()
	stopServerDependencies(registryClient, registryService, resultsStore, manager, &broadcastWg, wsServer, streamServer)
	cleanupOnError = false

	logging.Info("Server stopped")
	return exitCode
}

func startHTTPServer(cfg *config.Config, srv *http.Server, srvErrCh chan<- error) {
	go func() {
		fields := []logging.Field{
			{Key: "address", Value: cfg.BindAddress + ":" + cfg.Port},
			{Key: "tcp_test", Value: cfg.GetTCPTestAddress()},
			{Key: "udp_test", Value: cfg.GetUDPTestAddress()},
		}
		logging.Info("Server starting", fields...)
		var err error
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErrCh <- err
		}
	}()
}

func waitForShutdown(quit <-chan os.Signal, srvErrCh <-chan error) int {
	select {
	case sig := <-quit:
		logging.Info("Shutting down server...", logging.Field{Key: "signal", Value: sig.String()})
		return exitSuccess
	case err := <-srvErrCh:
		logging.Error("Server failed", logging.Field{Key: "error", Value: err})
		return exitFailure
	}
}

func shutdownHTTPServer(srv *http.Server, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logging.Error("Server shutdown error", logging.Field{Key: "error", Value: err})
	}
}

func stopServerDependencies(
	registryClient *registry.Client,
	registryService *registry.Service,
	resultsStore *results.Store,
	manager *stream.Manager,
	broadcastWg *sync.WaitGroup,
	wsServer *websocket.Server,
	streamServer *stream.Server,
) {
	if registryClient != nil {
		registryClient.Stop()
	}
	if registryService != nil {
		registryService.Stop()
	}
	if resultsStore != nil {
		resultsStore.Close()
	}
	if manager != nil {
		manager.Stop()
	}
	if broadcastWg != nil {
		broadcastWg.Wait()
	}
	if wsServer != nil {
		wsServer.Close()
	}
	if streamServer != nil {
		_ = streamServer.Close()
	}
}

func buildServerFlagSet(cfg *config.Config) (*flag.FlagSet, *serverFlagValues) {
	fs := flag.NewFlagSet("openbyte server", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: openbyte server [flags]\n\n")
		fmt.Fprintf(os.Stdout, "Server flags (override environment variables when set):\n")
		fs.SetOutput(os.Stdout)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}

	fv := &serverFlagValues{
		port:               fs.String("port", cfg.Port, "HTTP API port (env: PORT)"),
		bindAddress:        fs.String("bind-address", cfg.BindAddress, "Bind address (env: BIND_ADDRESS)"),
		tcpTestPort:        fs.Int("tcp-test-port", cfg.TCPTestPort, "TCP test port (env: TCP_TEST_PORT)"),
		udpTestPort:        fs.Int("udp-test-port", cfg.UDPTestPort, "UDP test port (env: UDP_TEST_PORT)"),
		serverID:           fs.String("server-id", cfg.ServerID, "Server ID (env: SERVER_ID)"),
		serverName:         fs.String("server-name", cfg.ServerName, "Server display name (env: SERVER_NAME)"),
		serverLocation:     fs.String("server-location", cfg.ServerLocation, "Server location (env: SERVER_LOCATION)"),
		serverRegion:       fs.String("server-region", cfg.ServerRegion, "Server region (env: SERVER_REGION)"),
		publicHost:         fs.String("public-host", cfg.PublicHost, "Public host for URLs (env: PUBLIC_HOST)"),
		capacityGbps:       fs.Int("capacity-gbps", cfg.CapacityGbps, "Capacity in Gbps (env: CAPACITY_GBPS)"),
		maxConcurrentTests: fs.Int("max-concurrent-tests", cfg.MaxConcurrentTests, "Max concurrent tests (env: MAX_CONCURRENT_TESTS)"),
		maxConcurrentPerIP: fs.Int("max-concurrent-per-ip", cfg.MaxConcurrentPerIP, "Max concurrent tests per IP (env: MAX_CONCURRENT_PER_IP)"),
		maxStreams:         fs.Int("max-streams", cfg.MaxStreams, "Max streams per test, 1-64 (env: MAX_STREAMS)"),
		maxTestDuration:    fs.String("max-test-duration", cfg.MaxTestDuration.String(), "Max test duration, e.g. 300s (env: MAX_TEST_DURATION)"),
		rateLimitPerIP:     fs.Int("rate-limit-per-ip", cfg.RateLimitPerIP, "Per-IP rate limit per minute (env: RATE_LIMIT_PER_IP)"),
		globalRateLimit:    fs.Int("global-rate-limit", cfg.GlobalRateLimit, "Global rate limit per minute (env: GLOBAL_RATE_LIMIT)"),
		allowedOrigins:     fs.String("allowed-origins", strings.Join(cfg.AllowedOrigins, ","), "Comma-separated allowed origins (env: ALLOWED_ORIGINS)"),
		trustProxyHeaders:  fs.Bool("trust-proxy-headers", cfg.TrustProxyHeaders, "Trust proxy headers (env: TRUST_PROXY_HEADERS)"),
		trustedProxyCIDRs:  fs.String("trusted-proxy-cidrs", strings.Join(cfg.TrustedProxyCIDRs, ","), "Comma-separated trusted proxy CIDRs (env: TRUSTED_PROXY_CIDRS)"),
		dataDir:            fs.String("data-dir", cfg.DataDir, "Data directory (env: DATA_DIR)"),
		maxStoredResults:   fs.Int("max-stored-results", cfg.MaxStoredResults, "Max stored results (env: MAX_STORED_RESULTS)"),
		webRoot:            fs.String("web-root", cfg.WebRoot, "Static web root override (env: WEB_ROOT)"),
		pprofEnabled:       fs.Bool("pprof-enabled", cfg.PprofEnabled, "Enable pprof server (env: PPROF_ENABLED)"),
		pprofAddress:       fs.String("pprof-addr", cfg.PprofAddress, "Pprof address (env: PPROF_ADDR)"),
		perfStatsInterval:  fs.String("perf-stats-interval", cfg.PerfStatsInterval.String(), "Runtime stats interval, e.g. 10s (env: PERF_STATS_INTERVAL)"),
		runtimeMetrics:     fs.Bool("runtime-metrics", cfg.RuntimeMetrics, "Enable runtime metrics endpoint /debug/runtime-metrics (env: RUNTIME_METRICS_ENABLED)"),
		registryEnabled:    fs.Bool("registry-enabled", cfg.RegistryEnabled, "Enable registry client mode (env: REGISTRY_ENABLED)"),
		registryMode:       fs.Bool("registry-mode", cfg.RegistryMode, "Enable registry server mode (env: REGISTRY_MODE)"),
		registryURL:        fs.String("registry-url", cfg.RegistryURL, "Registry URL (env: REGISTRY_URL)"),
		registryAPIKey:     fs.String("registry-api-key", cfg.RegistryAPIKey, "Registry API key (env: REGISTRY_API_KEY)"),
		registryInterval:   fs.String("registry-interval", cfg.RegistryInterval.String(), "Registry heartbeat interval, e.g. 30s (env: REGISTRY_INTERVAL)"),
		registryServerTTL:  fs.String("registry-server-ttl", cfg.RegistryServerTTL.String(), "Registry server TTL, e.g. 60s (env: REGISTRY_SERVER_TTL)"),
	}
	return fs, fv
}

func applyServerFlagOverrides(cfg *config.Config, fs *flag.FlagSet, fv *serverFlagValues) error {
	applyCSV := func(raw string) []string {
		if strings.TrimSpace(raw) == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	parseDuration := func(key string, raw string) (time.Duration, error) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return 0, fmt.Errorf("invalid --%s %q: %w", key, raw, err)
		}
		return d, nil
	}

	overrides := map[string]func() error{
		"port":                  func() error { cfg.Port = *fv.port; return nil },
		"bind-address":          func() error { cfg.BindAddress = *fv.bindAddress; return nil },
		"tcp-test-port":         func() error { cfg.TCPTestPort = *fv.tcpTestPort; return nil },
		"udp-test-port":         func() error { cfg.UDPTestPort = *fv.udpTestPort; return nil },
		"server-id":             func() error { cfg.ServerID = *fv.serverID; return nil },
		"server-name":           func() error { cfg.ServerName = *fv.serverName; return nil },
		"server-location":       func() error { cfg.ServerLocation = *fv.serverLocation; return nil },
		"server-region":         func() error { cfg.ServerRegion = *fv.serverRegion; return nil },
		"public-host":           func() error { cfg.PublicHost = *fv.publicHost; return nil },
		"capacity-gbps":         func() error { cfg.CapacityGbps = *fv.capacityGbps; return nil },
		"max-concurrent-tests":  func() error { cfg.MaxConcurrentTests = *fv.maxConcurrentTests; return nil },
		"max-concurrent-per-ip": func() error { cfg.MaxConcurrentPerIP = *fv.maxConcurrentPerIP; return nil },
		"max-streams":           func() error { cfg.MaxStreams = *fv.maxStreams; return nil },
		"max-test-duration": func() error {
			d, err := parseDuration("max-test-duration", *fv.maxTestDuration)
			if err != nil {
				return err
			}
			cfg.MaxTestDuration = d
			return nil
		},
		"rate-limit-per-ip":   func() error { cfg.RateLimitPerIP = *fv.rateLimitPerIP; return nil },
		"global-rate-limit":   func() error { cfg.GlobalRateLimit = *fv.globalRateLimit; return nil },
		"allowed-origins":     func() error { cfg.AllowedOrigins = applyCSV(*fv.allowedOrigins); return nil },
		"trust-proxy-headers": func() error { cfg.TrustProxyHeaders = *fv.trustProxyHeaders; return nil },
		"trusted-proxy-cidrs": func() error { cfg.TrustedProxyCIDRs = applyCSV(*fv.trustedProxyCIDRs); return nil },
		"data-dir":            func() error { cfg.DataDir = *fv.dataDir; return nil },
		"max-stored-results":  func() error { cfg.MaxStoredResults = *fv.maxStoredResults; return nil },
		"web-root":            func() error { cfg.WebRoot = *fv.webRoot; return nil },
		"pprof-enabled":       func() error { cfg.PprofEnabled = *fv.pprofEnabled; return nil },
		"pprof-addr":          func() error { cfg.PprofAddress = *fv.pprofAddress; return nil },
		"perf-stats-interval": func() error {
			d, err := parseDuration("perf-stats-interval", *fv.perfStatsInterval)
			if err != nil {
				return err
			}
			cfg.PerfStatsInterval = d
			return nil
		},
		"runtime-metrics":  func() error { cfg.RuntimeMetrics = *fv.runtimeMetrics; return nil },
		"registry-enabled": func() error { cfg.RegistryEnabled = *fv.registryEnabled; return nil },
		"registry-mode":    func() error { cfg.RegistryMode = *fv.registryMode; return nil },
		"registry-url":     func() error { cfg.RegistryURL = *fv.registryURL; return nil },
		"registry-api-key": func() error { cfg.RegistryAPIKey = *fv.registryAPIKey; return nil },
		"registry-interval": func() error {
			d, err := parseDuration("registry-interval", *fv.registryInterval)
			if err != nil {
				return err
			}
			cfg.RegistryInterval = d
			return nil
		},
		"registry-server-ttl": func() error {
			d, err := parseDuration("registry-server-ttl", *fv.registryServerTTL)
			if err != nil {
				return err
			}
			cfg.RegistryServerTTL = d
			return nil
		},
	}

	var applyErr error
	fs.Visit(func(f *flag.Flag) {
		if applyErr != nil {
			return
		}
		override, ok := overrides[f.Name]
		if !ok {
			return
		}
		if err := override(); err != nil {
			applyErr = err
		}
	})
	if applyErr != nil {
		return applyErr
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return fmt.Errorf("invalid --port %q: must be a number", cfg.Port)
	}
	return nil
}

func broadcastMetrics(manager *stream.Manager, wsServer *websocket.Server) {
	updateCh := manager.GetMetricsUpdateChannel()

	for update := range updateCh {
		if update.State.Status == types.StreamStatusRunning ||
			update.State.Status == types.StreamStatusStarting ||
			update.State.Status == types.StreamStatusCompleted ||
			update.State.Status == types.StreamStatusFailed {
			wsServer.BroadcastMetrics(update.StreamID, update.State)
		}
	}
}
