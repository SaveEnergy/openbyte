package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	BindAddress string

	TCPTestPort int
	UDPTestPort int

	ServerID       string
	ServerName     string
	ServerLocation string
	ServerRegion   string
	PublicHost     string
	CapacityGbps   int

	MaxConcurrentTests int
	MaxTestDuration    time.Duration
	MaxStreams         int

	TCPBufferSize int
	UDPBufferSize int

	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	PprofEnabled      bool
	PprofAddress      string
	PerfStatsInterval time.Duration

	RateLimitPerIP     int
	MaxConcurrentPerIP int
	GlobalRateLimit    int

	TrustProxyHeaders bool
	TrustedProxyCIDRs []string
	AllowedOrigins    []string

	TestRetentionPeriod   time.Duration
	WebSocketPingInterval time.Duration
	MetricsUpdateInterval time.Duration

	WebRoot          string
	DataDir          string
	MaxStoredResults int

	RegistryEnabled  bool
	RegistryURL      string
	RegistryAPIKey   string
	RegistryInterval time.Duration

	RegistryMode      bool
	RegistryServerTTL time.Duration

	TLSCertFile string
	TLSKeyFile  string
	TLSAutoGen  bool // Auto-generate self-signed cert for dev
}

func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		Port:                  "8080",
		BindAddress:           "0.0.0.0",
		TCPTestPort:           8081,
		UDPTestPort:           8082,
		ServerID:              hostname,
		ServerName:            "OpenByte Server",
		ServerLocation:        "Unknown",
		ServerRegion:          "",
		PublicHost:            "",
		CapacityGbps:          25,
		MaxConcurrentTests:    10,
		MaxTestDuration:       300 * time.Second,
		MaxStreams:            32,
		TCPBufferSize:         64 * 1024,
		UDPBufferSize:         1500,
		ReadTimeout:           0,                // disabled; upload handlers manage own body deadline
		ReadHeaderTimeout:     15 * time.Second, // protects against slowloris
		WriteTimeout:          0,                // disabled; streaming endpoints manage own duration
		IdleTimeout:           60 * time.Second,
		PprofEnabled:          false,
		PprofAddress:          "127.0.0.1:6060",
		PerfStatsInterval:     0,
		RateLimitPerIP:        100,
		MaxConcurrentPerIP:    10,
		GlobalRateLimit:       1000,
		TrustProxyHeaders:     false,
		TrustedProxyCIDRs:     nil,
		AllowedOrigins:        []string{"*"},
		TestRetentionPeriod:   1 * time.Hour,
		WebSocketPingInterval: 30 * time.Second,
		MetricsUpdateInterval: 1 * time.Second,
		WebRoot:               "",
		DataDir:               "./data",
		MaxStoredResults:      10000,
		RegistryEnabled:       false,
		RegistryURL:           "",
		RegistryAPIKey:        "",
		RegistryInterval:      30 * time.Second,
		RegistryMode:          false,
		RegistryServerTTL:     60 * time.Second,
		TLSCertFile:           "",
		TLSKeyFile:            "",
		TLSAutoGen:            true, // Auto-generate for dev by default
	}
}

func (c *Config) LoadFromEnv() error {
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
	if cap := os.Getenv("CAPACITY_GBPS"); cap != "" {
		g, err := strconv.Atoi(cap)
		if err != nil || g <= 0 {
			return fmt.Errorf("invalid CAPACITY_GBPS %q: must be a positive integer", cap)
		}
		c.CapacityGbps = g
	}

	if max := os.Getenv("MAX_CONCURRENT_TESTS"); max != "" {
		m, err := strconv.Atoi(max)
		if err != nil || m <= 0 {
			return fmt.Errorf("invalid MAX_CONCURRENT_TESTS %q: must be a positive integer", max)
		}
		c.MaxConcurrentTests = m
	}
	if max := os.Getenv("MAX_STREAMS"); max != "" {
		m, err := strconv.Atoi(max)
		if err != nil || m <= 0 || m > 64 {
			return fmt.Errorf("invalid MAX_STREAMS %q: must be 1-64", max)
		}
		c.MaxStreams = m
	}
	if dur := os.Getenv("MAX_TEST_DURATION"); dur != "" {
		d, err := time.ParseDuration(dur)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid MAX_TEST_DURATION %q: must be a positive duration (e.g. 300s)", dur)
		}
		c.MaxTestDuration = d
	}

	if enabled := os.Getenv("PPROF_ENABLED"); enabled == "true" || enabled == "1" {
		c.PprofEnabled = true
	}
	if addr := os.Getenv("PPROF_ADDR"); addr != "" {
		c.PprofAddress = addr
	}
	if interval := os.Getenv("PERF_STATS_INTERVAL"); interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid PERF_STATS_INTERVAL %q: must be a positive duration (e.g. 10s)", interval)
		}
		c.PerfStatsInterval = d
	}

	if limit := os.Getenv("RATE_LIMIT_PER_IP"); limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil || l <= 0 {
			return fmt.Errorf("invalid RATE_LIMIT_PER_IP %q: must be a positive integer", limit)
		}
		c.RateLimitPerIP = l
	}
	if limit := os.Getenv("GLOBAL_RATE_LIMIT"); limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil || l <= 0 {
			return fmt.Errorf("invalid GLOBAL_RATE_LIMIT %q: must be a positive integer", limit)
		}
		c.GlobalRateLimit = l
	}
	if limit := os.Getenv("MAX_CONCURRENT_PER_IP"); limit != "" {
		l, err := strconv.Atoi(limit)
		if err != nil || l <= 0 {
			return fmt.Errorf("invalid MAX_CONCURRENT_PER_IP %q: must be a positive integer", limit)
		}
		c.MaxConcurrentPerIP = l
	}
	if trust := os.Getenv("TRUST_PROXY_HEADERS"); trust == "true" || trust == "1" {
		c.TrustProxyHeaders = true
	}
	if cidrs := os.Getenv("TRUSTED_PROXY_CIDRS"); cidrs != "" {
		entries := strings.Split(cidrs, ",")
		c.TrustedProxyCIDRs = make([]string, 0, len(entries))
		for _, entry := range entries {
			value := strings.TrimSpace(entry)
			if value != "" {
				c.TrustedProxyCIDRs = append(c.TrustedProxyCIDRs, value)
			}
		}
	}
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		entries := strings.Split(origins, ",")
		c.AllowedOrigins = make([]string, 0, len(entries))
		for _, entry := range entries {
			value := strings.TrimSpace(entry)
			if value != "" {
				c.AllowedOrigins = append(c.AllowedOrigins, value)
			}
		}
	}
	if webRoot := os.Getenv("WEB_ROOT"); webRoot != "" {
		c.WebRoot = webRoot
	}
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		c.DataDir = dataDir
	}
	if max := os.Getenv("MAX_STORED_RESULTS"); max != "" {
		m, err := strconv.Atoi(max)
		if err != nil || m <= 0 {
			return fmt.Errorf("invalid MAX_STORED_RESULTS %q: must be a positive integer", max)
		}
		c.MaxStoredResults = m
	}

	if enabled := os.Getenv("REGISTRY_ENABLED"); enabled == "true" || enabled == "1" {
		c.RegistryEnabled = true
	}
	if url := os.Getenv("REGISTRY_URL"); url != "" {
		c.RegistryURL = url
	}
	if key := os.Getenv("REGISTRY_API_KEY"); key != "" {
		c.RegistryAPIKey = key
	}
	if interval := os.Getenv("REGISTRY_INTERVAL"); interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid REGISTRY_INTERVAL %q: must be a positive duration (e.g. 30s)", interval)
		}
		c.RegistryInterval = d
	}

	if mode := os.Getenv("REGISTRY_MODE"); mode == "true" || mode == "1" {
		c.RegistryMode = true
	}
	if ttl := os.Getenv("REGISTRY_SERVER_TTL"); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil || d <= 0 {
			return fmt.Errorf("invalid REGISTRY_SERVER_TTL %q: must be a positive duration (e.g. 60s)", ttl)
		}
		c.RegistryServerTTL = d
	}

	if cert := os.Getenv("TLS_CERT_FILE"); cert != "" {
		c.TLSCertFile = cert
	}
	if key := os.Getenv("TLS_KEY_FILE"); key != "" {
		c.TLSKeyFile = key
	}
	if autoGen := os.Getenv("TLS_AUTO_GEN"); autoGen == "false" || autoGen == "0" {
		c.TLSAutoGen = false
	}

	return nil
}

func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("invalid port %q: must be 1-65535", c.Port)
	}
	if c.TCPTestPort <= 0 || c.TCPTestPort > 65535 {
		return fmt.Errorf("invalid TCP test port: %d", c.TCPTestPort)
	}
	if c.UDPTestPort <= 0 || c.UDPTestPort > 65535 {
		return fmt.Errorf("invalid UDP test port: %d", c.UDPTestPort)
	}
	if c.TCPTestPort == c.UDPTestPort {
		return fmt.Errorf("TCP and UDP test ports cannot be the same")
	}
	httpPort, _ := strconv.Atoi(c.Port)
	if httpPort == c.TCPTestPort {
		return fmt.Errorf("HTTP port (%s) and TCP test port (%d) cannot be the same", c.Port, c.TCPTestPort)
	}
	if httpPort == c.UDPTestPort {
		return fmt.Errorf("HTTP port (%s) and UDP test port (%d) cannot be the same", c.Port, c.UDPTestPort)
	}
	if c.MaxConcurrentTests <= 0 {
		return fmt.Errorf("max concurrent tests must be > 0")
	}
	if c.MaxTestDuration <= 0 {
		return fmt.Errorf("max test duration must be > 0")
	}
	if c.MaxStreams <= 0 || c.MaxStreams > 64 {
		return fmt.Errorf("max streams must be 1-64")
	}
	if c.PprofEnabled && c.PprofAddress == "" {
		return fmt.Errorf("pprof address cannot be empty when enabled")
	}
	// WebRoot empty means use embedded assets; no validation needed
	if c.RateLimitPerIP <= 0 {
		return fmt.Errorf("rate limit per IP must be > 0")
	}
	if c.GlobalRateLimit <= 0 {
		return fmt.Errorf("global rate limit must be > 0")
	}
	if c.GlobalRateLimit < c.RateLimitPerIP {
		return fmt.Errorf("global rate limit must be >= rate limit per IP")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}
	if c.MaxStoredResults <= 0 {
		return fmt.Errorf("max stored results must be > 0")
	}
	if c.TrustProxyHeaders && len(c.TrustedProxyCIDRs) > 0 {
		for _, entry := range c.TrustedProxyCIDRs {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("invalid trusted proxy CIDR: %s", entry)
			}
		}
	}
	return nil
}

// MaxConcurrentHTTP returns the concurrent download/upload limit for
// HTTP speed tests, derived from CapacityGbps. Each HTTP stream can
// push ~150 Mbps on a single TCP connection, so we allow roughly
// 8 slots per Gbps of declared capacity with a floor of 50.
func (c *Config) MaxConcurrentHTTP() int {
	limit := c.CapacityGbps * 8
	if limit < 50 {
		limit = 50
	}
	return limit
}

func (c *Config) GetTCPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.TCPTestPort)
}

func (c *Config) GetUDPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.UDPTestPort)
}
