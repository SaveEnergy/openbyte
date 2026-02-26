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
	RuntimeMetrics    bool

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

const (
	envTrue  = "true"
	envOne   = "1"
	EnvDebug = "debug" // exported for cmd/server
	envFalse = "false"
	envZero  = "0"
)

func DefaultConfig() *Config {
	hostname := defaultServerID()
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
		UDPBufferSize:         1400,
		ReadTimeout:           0,                // disabled; upload handlers manage own body deadline
		ReadHeaderTimeout:     15 * time.Second, // protects against slowloris
		WriteTimeout:          0,                // disabled; streaming endpoints manage own duration
		IdleTimeout:           60 * time.Second,
		PprofEnabled:          false,
		PprofAddress:          "127.0.0.1:6060",
		PerfStatsInterval:     0,
		RuntimeMetrics:        false,
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

func defaultServerID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "openbyte-server"
	}
	return hostname
}

func (c *Config) LoadFromEnv() error {
	if err := c.loadCoreEnv(); err != nil {
		return err
	}
	if err := c.loadRuntimeEnv(); err != nil {
		return err
	}
	if err := c.loadLimitsAndNetworkEnv(); err != nil {
		return err
	}
	if err := c.loadStorageEnv(); err != nil {
		return err
	}
	if err := c.loadRegistryEnv(); err != nil {
		return err
	}
	c.loadTLSEnv()
	return nil
}

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

func (c *Config) loadCoreEnv() error {
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

func (c *Config) Validate() error {
	if err := c.validatePorts(); err != nil {
		return err
	}
	if err := c.validateLimits(); err != nil {
		return err
	}
	if err := c.validateProxyAndStorage(); err != nil {
		return err
	}
	if err := c.validateRegistry(); err != nil {
		return err
	}
	if err := c.validateTLS(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validatePorts() error {
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	httpPort, err := strconv.Atoi(c.Port)
	if err != nil || httpPort < 1 || httpPort > 65535 {
		return fmt.Errorf("invalid port %q: must be 1-65535", c.Port)
	}
	if err := validateTestPort("TCP", c.TCPTestPort); err != nil {
		return err
	}
	if err := validateTestPort("UDP", c.UDPTestPort); err != nil {
		return err
	}
	return validatePortCollisions(c.Port, httpPort, c.TCPTestPort, c.UDPTestPort)
}

func validateTestPort(name string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid %s test port: %d", name, port)
	}
	return nil
}

func validatePortCollisions(httpPortRaw string, httpPort, tcpPort, udpPort int) error {
	if tcpPort == udpPort {
		return fmt.Errorf("TCP and UDP test ports cannot be the same")
	}
	if httpPort == tcpPort {
		return fmt.Errorf("HTTP port (%s) and TCP test port (%d) cannot be the same", httpPortRaw, tcpPort)
	}
	if httpPort == udpPort {
		return fmt.Errorf("HTTP port (%s) and UDP test port (%d) cannot be the same", httpPortRaw, udpPort)
	}
	return nil
}

func (c *Config) validateLimits() error {
	if c.MaxConcurrentTests <= 0 {
		return fmt.Errorf("max concurrent tests must be > 0")
	}
	if c.MaxTestDuration <= 0 {
		return fmt.Errorf("max test duration must be > 0")
	}
	if c.MaxStreams <= 0 || c.MaxStreams > 64 {
		return fmt.Errorf("max streams must be 1-64")
	}
	if c.CapacityGbps <= 0 {
		return fmt.Errorf("capacity gbps must be > 0")
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
	if c.MaxConcurrentPerIP <= 0 {
		return fmt.Errorf("max concurrent per IP must be > 0")
	}
	if c.MaxConcurrentPerIP > c.MaxConcurrentTests {
		return fmt.Errorf("max concurrent per IP must be <= max concurrent tests")
	}
	return nil
}

func (c *Config) validateProxyAndStorage() error {
	if c.DataDir == "" {
		return fmt.Errorf("data directory cannot be empty")
	}
	if c.MaxStoredResults <= 0 {
		return fmt.Errorf("max stored results must be > 0")
	}
	if len(c.TrustedProxyCIDRs) > 0 {
		for _, entry := range c.TrustedProxyCIDRs {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("invalid trusted proxy CIDR: %s", entry)
			}
		}
	}
	if c.TrustProxyHeaders && len(c.TrustedProxyCIDRs) == 0 {
		return fmt.Errorf("trusted proxy CIDRs required when trust proxy headers is enabled")
	}
	return nil
}

func (c *Config) validateRegistry() error {
	if c.RegistryEnabled && c.RegistryInterval <= 0 {
		return fmt.Errorf("registry interval must be > 0 when registry is enabled")
	}
	if c.RegistryEnabled && strings.TrimSpace(c.RegistryURL) == "" {
		return fmt.Errorf("registry URL required when registry is enabled")
	}
	return nil
}

func (c *Config) validateTLS() error {
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must both be set or both be empty")
	}
	if c.TLSCertFile != "" {
		if _, err := os.Stat(c.TLSCertFile); err != nil {
			return fmt.Errorf("TLS cert file not accessible: %w", err)
		}
		if _, err := os.Stat(c.TLSKeyFile); err != nil {
			return fmt.Errorf("TLS key file not accessible: %w", err)
		}
	}
	return nil
}

// MaxConcurrentHTTP returns the concurrent download/upload limit for
// HTTP speed tests, derived from CapacityGbps. Each HTTP stream can
// push ~150 Mbps on a single TCP connection, so we allow roughly
// 8 slots per Gbps of declared capacity with a floor of 50.
func (c *Config) MaxConcurrentHTTP() int {
	limit := max(c.CapacityGbps*8, 50)
	return limit
}

func (c *Config) GetTCPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.TCPTestPort)
}

func (c *Config) GetUDPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.UDPTestPort)
}
