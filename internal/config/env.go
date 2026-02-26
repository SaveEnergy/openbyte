package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envTrue  = "true"
	envOne   = "1"
	EnvDebug = "debug" // exported for cmd/server
	envFalse = "false"
	envZero  = "0"
)

func envBool(name string) bool {
	val := os.Getenv(name)
	return val == envTrue || val == envOne
}

func envCSV(name string) []string {
	raw := os.Getenv(name)
	if raw == "" {
		return nil
	}
	entries := strings.Split(raw, ",")
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		value := strings.TrimSpace(entry)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func parsePositiveIntEnv(name string) (int, bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, true, fmt.Errorf("invalid %s %q: must be a positive integer", name, raw)
	}
	return v, true, nil
}

func parseDurationEnv(name string) (time.Duration, bool, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, true, fmt.Errorf("invalid %s %q: must be a positive duration", name, raw)
	}
	return d, true, nil
}

func defaultServerID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "openbyte-server"
	}
	return hostname
}

func (c *Config) loadCoreEnv() error {
	if err := c.loadCoreEnvPortsAndBind(); err != nil {
		return err
	}
	if err := c.loadCoreEnvServerMeta(); err != nil {
		return err
	}
	return c.loadCoreEnvCapacityAndLimits()
}

func (c *Config) loadCoreEnvPortsAndBind() error {
	if port := os.Getenv("PORT"); port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			return fmt.Errorf("invalid PORT %q: must be a number", port)
		}
		c.Port = port
	}
	if addr := os.Getenv("BIND_ADDRESS"); addr != "" {
		c.BindAddress = addr
	}
	if port := os.Getenv("TCP_TEST_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("invalid TCP_TEST_PORT %q: %w", port, err)
		}
		c.TCPTestPort = p
	}
	if port := os.Getenv("UDP_TEST_PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("invalid UDP_TEST_PORT %q: %w", port, err)
		}
		c.UDPTestPort = p
	}
	return nil
}

func (c *Config) loadCoreEnvServerMeta() error {
	if id := os.Getenv("SERVER_ID"); id != "" {
		c.ServerID = id
	}
	if name := os.Getenv("SERVER_NAME"); name != "" {
		c.ServerName = name
	}
	if loc := os.Getenv("SERVER_LOCATION"); loc != "" {
		c.ServerLocation = loc
	}
	if region := os.Getenv("SERVER_REGION"); region != "" {
		c.ServerRegion = region
	}
	if host := os.Getenv("PUBLIC_HOST"); host != "" {
		c.PublicHost = host
	}
	return nil
}

func (c *Config) loadCoreEnvCapacityAndLimits() error {
	if capRaw := os.Getenv("CAPACITY_GBPS"); capRaw != "" {
		g, err := strconv.Atoi(capRaw)
		if err != nil || g <= 0 {
			return fmt.Errorf("invalid CAPACITY_GBPS %q: must be a positive integer", capRaw)
		}
		c.CapacityGbps = g
	}
	if max, ok, err := parsePositiveIntEnv("MAX_CONCURRENT_TESTS"); err != nil {
		return err
	} else if ok {
		c.MaxConcurrentTests = max
	}
	if maxRaw := os.Getenv("MAX_STREAMS"); maxRaw != "" {
		m, err := strconv.Atoi(maxRaw)
		if err != nil || m <= 0 || m > 64 {
			return fmt.Errorf("invalid MAX_STREAMS %q: must be 1-64", maxRaw)
		}
		c.MaxStreams = m
	}
	if durRaw := os.Getenv("MAX_TEST_DURATION"); durRaw != "" {
		d, err := time.ParseDuration(durRaw)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid MAX_TEST_DURATION %q: must be a positive duration (e.g. 300s)", durRaw)
		}
		c.MaxTestDuration = d
	}
	return nil
}

func (c *Config) loadRuntimeEnv() error {
	c.PprofEnabled = c.PprofEnabled || envBool("PPROF_ENABLED")
	if addr := os.Getenv("PPROF_ADDR"); addr != "" {
		c.PprofAddress = addr
	}
	if intervalRaw := os.Getenv("PERF_STATS_INTERVAL"); intervalRaw != "" {
		d, err := time.ParseDuration(intervalRaw)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid PERF_STATS_INTERVAL %q: must be a positive duration (e.g. 10s)", intervalRaw)
		}
		c.PerfStatsInterval = d
	}
	c.RuntimeMetrics = c.RuntimeMetrics || envBool("RUNTIME_METRICS_ENABLED")
	return nil
}

func (c *Config) loadLimitsAndNetworkEnv() error {
	if limit, ok, err := parsePositiveIntEnv("RATE_LIMIT_PER_IP"); err != nil {
		return err
	} else if ok {
		c.RateLimitPerIP = limit
	}
	if limit, ok, err := parsePositiveIntEnv("GLOBAL_RATE_LIMIT"); err != nil {
		return err
	} else if ok {
		c.GlobalRateLimit = limit
	}
	if limit, ok, err := parsePositiveIntEnv("MAX_CONCURRENT_PER_IP"); err != nil {
		return err
	} else if ok {
		c.MaxConcurrentPerIP = limit
	}
	c.TrustProxyHeaders = c.TrustProxyHeaders || envBool("TRUST_PROXY_HEADERS")
	if cidrs := envCSV("TRUSTED_PROXY_CIDRS"); cidrs != nil {
		c.TrustedProxyCIDRs = cidrs
	}
	if origins := envCSV("ALLOWED_ORIGINS"); origins != nil {
		c.AllowedOrigins = origins
	}
	if webRoot := os.Getenv("WEB_ROOT"); webRoot != "" {
		c.WebRoot = webRoot
	}
	return nil
}

func (c *Config) loadStorageEnv() error {
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		c.DataDir = dataDir
	}
	if max, ok, err := parsePositiveIntEnv("MAX_STORED_RESULTS"); err != nil {
		return err
	} else if ok {
		c.MaxStoredResults = max
	}
	return nil
}

func (c *Config) loadRegistryEnv() error {
	c.RegistryEnabled = c.RegistryEnabled || envBool("REGISTRY_ENABLED")
	if u := os.Getenv("REGISTRY_URL"); u != "" {
		c.RegistryURL = u
	}
	if key := os.Getenv("REGISTRY_API_KEY"); key != "" {
		c.RegistryAPIKey = key
	}
	if d, ok, err := parseDurationEnv("REGISTRY_INTERVAL"); err != nil {
		return fmt.Errorf("%w (e.g. 30s)", err)
	} else if ok {
		c.RegistryInterval = d
	}
	c.RegistryMode = c.RegistryMode || envBool("REGISTRY_MODE")
	if d, ok, err := parseDurationEnv("REGISTRY_SERVER_TTL"); err != nil {
		return fmt.Errorf("%w (e.g. 60s)", err)
	} else if ok {
		c.RegistryServerTTL = d
	}
	return nil
}

func (c *Config) loadTLSEnv() {
	if cert := os.Getenv("TLS_CERT_FILE"); cert != "" {
		c.TLSCertFile = cert
	}
	if key := os.Getenv("TLS_KEY_FILE"); key != "" {
		c.TLSKeyFile = key
	}
	if autoGen := os.Getenv("TLS_AUTO_GEN"); autoGen == envFalse || autoGen == envZero {
		c.TLSAutoGen = false
	}
}
