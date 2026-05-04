package server

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

const (
	flagMaxTestDuration   = "max-test-duration"
	flagPerfStatsInterval = "perf-stats-interval"
)

type serverFlagValues struct {
	port               *string
	bindAddress        *string
	serverName         *string
	publicHost         *string
	capacityGbps       *int
	maxConcurrentPerIP *int
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
		serverName:         fs.String("server-name", cfg.ServerName, "Display name for this server (env: SERVER_NAME)"),
		publicHost:         fs.String("public-host", cfg.PublicHost, "Public host for URLs (env: PUBLIC_HOST)"),
		capacityGbps:       fs.Int("capacity-gbps", cfg.CapacityGbps, "Capacity in Gbps (env: CAPACITY_GBPS)"),
		maxConcurrentPerIP: fs.Int("max-concurrent-per-ip", cfg.MaxConcurrentPerIP, "Max concurrent tests per IP (env: MAX_CONCURRENT_PER_IP)"),
		maxTestDuration:    fs.String(flagMaxTestDuration, cfg.MaxTestDuration.String(), "Max test duration, e.g. 300s (env: MAX_TEST_DURATION)"),
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
		perfStatsInterval:  fs.String(flagPerfStatsInterval, cfg.PerfStatsInterval.String(), "Runtime stats interval, e.g. 10s (env: PERF_STATS_INTERVAL)"),
		runtimeMetrics:     fs.Bool("runtime-metrics", cfg.RuntimeMetrics, "Enable runtime metrics endpoint /debug/runtime-metrics (env: RUNTIME_METRICS_ENABLED)"),
	}
	return fs, fv
}

func parseCSV(raw string) []string {
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

func parseFlagDuration(key, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid --%s %q: %w", key, raw, err)
	}
	return d, nil
}

func setFlagDuration(key, raw string, target *time.Duration) error {
	d, err := parseFlagDuration(key, raw)
	if err != nil {
		return err
	}
	*target = d
	return nil
}

func applyOverrideForFlag(name string, overrides map[string]func() error) error {
	override, ok := overrides[name]
	if !ok {
		return nil
	}
	return override()
}

func applyServerFlagOverrides(cfg *config.Config, fs *flag.FlagSet, fv *serverFlagValues) error {

	overrides := map[string]func() error{
		"port":                  func() error { cfg.Port = *fv.port; return nil },
		"bind-address":          func() error { cfg.BindAddress = *fv.bindAddress; return nil },
		"server-name":           func() error { cfg.ServerName = strings.TrimSpace(*fv.serverName); return nil },
		"public-host":           func() error { cfg.PublicHost = *fv.publicHost; return nil },
		"capacity-gbps":         func() error { cfg.CapacityGbps = *fv.capacityGbps; return nil },
		"max-concurrent-per-ip": func() error { cfg.MaxConcurrentPerIP = *fv.maxConcurrentPerIP; return nil },
		flagMaxTestDuration:     func() error { return setFlagDuration(flagMaxTestDuration, *fv.maxTestDuration, &cfg.MaxTestDuration) },
		"rate-limit-per-ip":     func() error { cfg.RateLimitPerIP = *fv.rateLimitPerIP; return nil },
		"global-rate-limit":     func() error { cfg.GlobalRateLimit = *fv.globalRateLimit; return nil },
		"allowed-origins":       func() error { cfg.AllowedOrigins = parseCSV(*fv.allowedOrigins); return nil },
		"trust-proxy-headers":   func() error { cfg.TrustProxyHeaders = *fv.trustProxyHeaders; return nil },
		"trusted-proxy-cidrs":   func() error { cfg.TrustedProxyCIDRs = parseCSV(*fv.trustedProxyCIDRs); return nil },
		"data-dir":              func() error { cfg.DataDir = *fv.dataDir; return nil },
		"max-stored-results":    func() error { cfg.MaxStoredResults = *fv.maxStoredResults; return nil },
		"web-root":              func() error { cfg.WebRoot = *fv.webRoot; return nil },
		"pprof-enabled":         func() error { cfg.PprofEnabled = *fv.pprofEnabled; return nil },
		"pprof-addr":            func() error { cfg.PprofAddress = *fv.pprofAddress; return nil },
		flagPerfStatsInterval: func() error {
			return setFlagDuration(flagPerfStatsInterval, *fv.perfStatsInterval, &cfg.PerfStatsInterval)
		},
		"runtime-metrics": func() error { cfg.RuntimeMetrics = *fv.runtimeMetrics; return nil },
	}

	var applyErr error
	fs.Visit(func(f *flag.Flag) {
		if applyErr != nil {
			return
		}
		if err := applyOverrideForFlag(f.Name, overrides); err != nil {
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
