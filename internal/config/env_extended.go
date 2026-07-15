package config

import (
	"os"
)

func (c *Config) loadRuntimeEnv() {
	c.PprofEnabled = c.PprofEnabled || envBool("PPROF_ENABLED")
	if addr := os.Getenv("PPROF_ADDR"); addr != "" {
		c.PprofAddress = addr
	}
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
	if autoGen := os.Getenv("TLS_AUTO_GEN"); autoGen != "" {
		c.TLSAutoGen = envBool("TLS_AUTO_GEN")
	}
	if http2Enabled := os.Getenv("HTTP2_ENABLED"); http2Enabled != "" {
		c.HTTP2Enabled = envBool("HTTP2_ENABLED")
	}
}
