package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/results"
)

type serverResources struct {
	resultsStore *results.Store
}

func (r *serverResources) stopAll(pprofServer *http.Server, stopStats func()) {
	if r.resultsStore != nil {
		r.resultsStore.Close()
	}
	shutdownPprofServer(pprofServer, 5*time.Second)
	stopStats()
}

func setupRuntimeResources(cfg *config.Config, version string, resources *serverResources) (http.Handler, error) {
	apiHandler := api.NewHandler()
	apiHandler.SetConfig(cfg)
	apiHandler.SetVersion(version)

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		logging.Error("Failed to create data directory", logging.Field{Key: "error", Value: err})
		return nil, err
	}
	var err error
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
	router.SetResultsHandler(results.NewHandler(resources.resultsStore))
	router.SetWebRoot(cfg.WebRoot)
	if cfg.RuntimeMetrics {
		router.SetRuntimeMetricsHandler(runtimeMetricsHandler())
	}

	return router.SetupRoutes(), nil
}

func startHTTPServer(cfg *config.Config, srv *http.Server, srvErrCh chan<- error) {
	go func() {
		logging.Info("Server starting", logging.Field{Key: "address", Value: cfg.BindAddress + ":" + cfg.Port})
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
