package server

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

func startPprofServer(cfg *config.Config) *http.Server {
	if cfg == nil || !cfg.PprofEnabled {
		return nil
	}

	srv := &http.Server{
		Addr:    cfg.PprofAddress,
		Handler: http.DefaultServeMux,
	}

	go func() {
		logging.Info("pprof server starting", logging.Field{Key: "address", Value: cfg.PprofAddress})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("pprof server failed", logging.Field{Key: "error", Value: err})
		}
	}()

	return srv
}

func shutdownPprofServer(srv *http.Server, timeout time.Duration) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logging.Warn("pprof server shutdown error", logging.Field{Key: "error", Value: err})
	}
}

func startRuntimeStatsLogger(cfg *config.Config) {
	if cfg == nil || cfg.PerfStatsInterval <= 0 {
		return
	}

	interval := cfg.PerfStatsInterval
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var mem runtime.MemStats
		for range ticker.C {
			runtime.ReadMemStats(&mem)
			gcStats := debug.GCStats{}
			debug.ReadGCStats(&gcStats)

			logging.Info("runtime stats",
				logging.Field{Key: "goroutines", Value: runtime.NumGoroutine()},
				logging.Field{Key: "heap_alloc_bytes", Value: mem.HeapAlloc},
				logging.Field{Key: "heap_inuse_bytes", Value: mem.HeapInuse},
				logging.Field{Key: "stack_inuse_bytes", Value: mem.StackInuse},
				logging.Field{Key: "gc_count", Value: mem.NumGC},
				logging.Field{Key: "gc_pause_total_ns", Value: mem.PauseTotalNs},
				logging.Field{Key: "last_gc", Value: gcStats.LastGC},
			)
		}
	}()
}
