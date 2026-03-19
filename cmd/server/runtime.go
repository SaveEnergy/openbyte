package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
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

type serverResources struct {
	streamServer    *stream.Server
	manager         *stream.Manager
	resultsStore    *results.Store
	wsServer        *websocket.Server
	registryService *registry.Service
	registryClient  *registry.Client
	broadcastWg     sync.WaitGroup
}

func (r *serverResources) stopAll(pprofServer *http.Server, stopStats func()) {
	stopServerDependencies(
		r.registryClient,
		r.registryService,
		r.resultsStore,
		r.manager,
		&r.broadcastWg,
		r.wsServer,
		r.streamServer,
	)
	shutdownPprofServer(pprofServer, 5*time.Second)
	stopStats()
}

func setupRuntimeResources(cfg *config.Config, version string, resources *serverResources) (http.Handler, error) {
	var err error
	resources.streamServer, err = stream.NewServer(cfg)
	if err != nil {
		logging.Error("Failed to start stream server", logging.Field{Key: "error", Value: err})
		return nil, err
	}
	logging.Info("Stream server started",
		logging.Field{Key: "tcp_port", Value: cfg.TCPTestPort},
		logging.Field{Key: "udp_port", Value: cfg.UDPTestPort})

	resources.manager = stream.NewManager(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)
	resources.manager.SetRetentionPeriod(cfg.TestRetentionPeriod)
	resources.manager.SetMetricsUpdateInterval(cfg.MetricsUpdateInterval)
	resources.manager.Start()

	apiHandler := api.NewHandler(resources.manager)
	apiHandler.SetConfig(cfg)
	apiHandler.SetVersion(version)
	resources.wsServer = websocket.NewServer()
	resources.wsServer.SetAllowedOrigins(cfg.AllowedOrigins)
	resources.wsServer.SetPingInterval(cfg.WebSocketPingInterval)
	resources.wsServer.SetConnectionLimits(cfg.MaxConcurrentTests, cfg.MaxConcurrentPerIP)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logging.Error("Failed to create data directory", logging.Field{Key: "error", Value: err})
		return nil, err
	}
	resources.resultsStore, err = results.New(cfg.DataDir+"/results.db", cfg.MaxStoredResults)
	if err != nil {
		logging.Error("Failed to open results store", logging.Field{Key: "error", Value: err})
		return nil, err
	}
	logging.Info("Results store opened",
		logging.Field{Key: "path", Value: cfg.DataDir + "/results.db"},
		logging.Field{Key: "max_results", Value: cfg.MaxStoredResults})

	router := api.NewRouter(apiHandler, cfg)
	router.SetRateLimiter(cfg)
	router.SetClientIPResolver(api.NewClientIPResolver(cfg))
	router.SetAllowedOrigins(cfg.AllowedOrigins)
	router.SetWebSocketHandler(resources.wsServer.HandleStream)
	router.SetResultsHandler(results.NewHandler(resources.resultsStore))
	router.SetWebRoot(cfg.WebRoot)
	if cfg.RuntimeMetrics {
		router.SetRuntimeMetricsHandler(runtimeMetricsHandler())
	}

	var registrars []api.RouteRegisterer
	if cfg.RegistryMode {
		logging.Info("Starting in registry mode")
		resources.registryService = registry.NewService(cfg.RegistryServerTTL, 30*time.Second)
		resources.registryService.Start()

		registryLogger := logging.NewLogger("registry")
		registryHandler := registry.NewHandler(resources.registryService, registryLogger, cfg.RegistryAPIKey)
		registrars = append(registrars, registryHandler)
	}

	muxRouter := router.SetupRoutes(registrars...)
	resources.broadcastWg.Go(func() {
		broadcastMetrics(resources.manager, resources.wsServer)
	})

	if cfg.RegistryEnabled && !cfg.RegistryMode {
		logger := logging.NewLogger("registry-client")
		resources.registryClient = registry.NewClient(cfg, logger)
		if err := resources.registryClient.Start(resources.manager.ActiveCount); err != nil {
			logging.Warn("Registry client failed to start", logging.Field{Key: "error", Value: err})
		}
	}
	return muxRouter, nil
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
