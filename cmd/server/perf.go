package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
	rtmetrics "runtime/metrics"
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
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

func startRuntimeStatsLogger(cfg *config.Config) func() {
	if cfg == nil || cfg.PerfStatsInterval <= 0 {
		return func() {}
	}

	interval := cfg.PerfStatsInterval
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var mem runtime.MemStats
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				runtime.ReadMemStats(&mem)
				gcStats := debug.GCStats{}
				debug.ReadGCStats(&gcStats)
				rm := collectRuntimeMetrics()

				logging.Info("runtime stats",
					logging.Field{Key: "goroutines", Value: runtime.NumGoroutine()},
					logging.Field{Key: "goroutines_runnable", Value: rm["/sched/goroutines/runnable:goroutines"]},
					logging.Field{Key: "goroutines_waiting", Value: rm["/sched/goroutines/waiting:goroutines"]},
					logging.Field{Key: "heap_alloc_bytes", Value: mem.HeapAlloc},
					logging.Field{Key: "heap_inuse_bytes", Value: mem.HeapInuse},
					logging.Field{Key: "stack_inuse_bytes", Value: mem.StackInuse},
					logging.Field{Key: "gc_count", Value: mem.NumGC},
					logging.Field{Key: "gc_pause_total_ns", Value: mem.PauseTotalNs},
					logging.Field{Key: "mutex_wait_total_seconds", Value: rm["/sync/mutex/wait/total:seconds"]},
					logging.Field{Key: "last_gc", Value: gcStats.LastGC},
				)
			}
		}
	}()
	return func() { close(stopCh) }
}

func runtimeMetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"runtime_metrics": collectRuntimeMetrics(),
		}); err != nil {
			logging.Warn("runtime metrics: encode response", logging.Field{Key: "error", Value: err})
		}
	}
}

func collectRuntimeMetrics() map[string]float64 {
	samples := []rtmetrics.Sample{
		{Name: "/sched/goroutines:goroutines"},
		{Name: "/sched/goroutines/runnable:goroutines"},
		{Name: "/sched/goroutines/waiting:goroutines"},
		{Name: "/sched/threads/total:threads"},
		{Name: "/gc/heap/live:bytes"},
		{Name: "/gc/heap/goal:bytes"},
		{Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/sync/mutex/wait/total:seconds"},
	}
	rtmetrics.Read(samples)

	out := make(map[string]float64, len(samples))
	for _, sample := range samples {
		switch sample.Value.Kind() {
		case rtmetrics.KindUint64:
			out[sample.Name] = float64(sample.Value.Uint64())
		case rtmetrics.KindFloat64:
			out[sample.Name] = sample.Value.Float64()
		}
	}
	return out
}
