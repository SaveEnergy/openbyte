package config

import (
	"fmt"
	"os"
	"time"
)

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
