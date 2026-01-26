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

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

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

	WebRoot string

	RegistryEnabled  bool
	RegistryURL      string
	RegistryAPIKey   string
	RegistryInterval time.Duration

	RegistryMode      bool
	RegistryServerTTL time.Duration

	// QUIC/HTTP3 settings
	QUICEnabled  bool
	QUICPort     int
	HTTP3Enabled bool
	HTTP3Port    int
	TLSCertFile  string
	TLSKeyFile   string
	TLSAutoGen   bool // Auto-generate self-signed cert for dev
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
		MaxStreams:            16,
		TCPBufferSize:         64 * 1024,
		UDPBufferSize:         1500,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          15 * time.Second,
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
		WebRoot:               "./web",
		RegistryEnabled:       false,
		RegistryURL:           "",
		RegistryAPIKey:        "",
		RegistryInterval:      30 * time.Second,
		RegistryMode:          false,
		RegistryServerTTL:     60 * time.Second,
		QUICEnabled:           false,
		QUICPort:              8083,
		HTTP3Enabled:          false,
		HTTP3Port:             8443,
		TLSCertFile:           "",
		TLSKeyFile:            "",
		TLSAutoGen:            true, // Auto-generate for dev by default
	}
}

func (c *Config) LoadFromEnv() error {
	if port := os.Getenv("PORT"); port != "" {
		c.Port = port
	}
	if addr := os.Getenv("BIND_ADDRESS"); addr != "" {
		c.BindAddress = addr
	}

	if port := os.Getenv("TCP_TEST_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.TCPTestPort = p
		}
	}
	if port := os.Getenv("UDP_TEST_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.UDPTestPort = p
		}
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
		if g, err := strconv.Atoi(cap); err == nil && g > 0 {
			c.CapacityGbps = g
		}
	}

	if max := os.Getenv("MAX_CONCURRENT_TESTS"); max != "" {
		if m, err := strconv.Atoi(max); err == nil && m > 0 {
			c.MaxConcurrentTests = m
		}
	}
	if max := os.Getenv("MAX_STREAMS"); max != "" {
		if m, err := strconv.Atoi(max); err == nil && m > 0 && m <= 16 {
			c.MaxStreams = m
		}
	}

	if enabled := os.Getenv("PPROF_ENABLED"); enabled == "true" || enabled == "1" {
		c.PprofEnabled = true
	}
	if addr := os.Getenv("PPROF_ADDR"); addr != "" {
		c.PprofAddress = addr
	}
	if interval := os.Getenv("PERF_STATS_INTERVAL"); interval != "" {
		if d, err := time.ParseDuration(interval); err == nil && d > 0 {
			c.PerfStatsInterval = d
		}
	}

	if limit := os.Getenv("RATE_LIMIT_PER_IP"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			c.RateLimitPerIP = l
		}
	}
	if limit := os.Getenv("MAX_CONCURRENT_PER_IP"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			c.MaxConcurrentPerIP = l
		}
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
		if d, err := time.ParseDuration(interval); err == nil && d > 0 {
			c.RegistryInterval = d
		}
	}

	if mode := os.Getenv("REGISTRY_MODE"); mode == "true" || mode == "1" {
		c.RegistryMode = true
	}
	if ttl := os.Getenv("REGISTRY_SERVER_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil && d > 0 {
			c.RegistryServerTTL = d
		}
	}

	// QUIC/HTTP3 settings
	if enabled := os.Getenv("QUIC_ENABLED"); enabled == "true" || enabled == "1" {
		c.QUICEnabled = true
	}
	if port := os.Getenv("QUIC_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			c.QUICPort = p
		}
	}
	if enabled := os.Getenv("HTTP3_ENABLED"); enabled == "true" || enabled == "1" {
		c.HTTP3Enabled = true
	}
	if port := os.Getenv("HTTP3_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			c.HTTP3Port = p
		}
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
	if c.TCPTestPort <= 0 || c.TCPTestPort > 65535 {
		return fmt.Errorf("invalid TCP test port: %d", c.TCPTestPort)
	}
	if c.UDPTestPort <= 0 || c.UDPTestPort > 65535 {
		return fmt.Errorf("invalid UDP test port: %d", c.UDPTestPort)
	}
	if c.TCPTestPort == c.UDPTestPort {
		return fmt.Errorf("TCP and UDP test ports cannot be the same")
	}
	if c.MaxConcurrentTests <= 0 {
		return fmt.Errorf("max concurrent tests must be > 0")
	}
	if c.MaxStreams <= 0 || c.MaxStreams > 16 {
		return fmt.Errorf("max streams must be 1-16")
	}
	if c.PprofEnabled && c.PprofAddress == "" {
		return fmt.Errorf("pprof address cannot be empty when enabled")
	}
	if c.WebRoot == "" {
		return fmt.Errorf("web root cannot be empty")
	}
	if c.RateLimitPerIP <= 0 {
		return fmt.Errorf("rate limit per IP must be > 0")
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

func (c *Config) GetTCPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.TCPTestPort)
}

func (c *Config) GetUDPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.UDPTestPort)
}

func (c *Config) GetQUICAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.QUICPort)
}

func (c *Config) GetHTTP3Address() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.HTTP3Port)
}
