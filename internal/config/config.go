package config

import "time"

const DefaultServerName = "openByte Server"

type Config struct {
	Port        string
	BindAddress string
	ServerName  string

	PublicHost   string
	CapacityGbps int

	MaxTestDuration time.Duration

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

	WebRoot          string
	DataDir          string
	MaxStoredResults int

	TLSCertFile string
	TLSKeyFile  string
	TLSAutoGen  bool // Auto-generate self-signed cert for dev
}

func DefaultConfig() *Config {
	return &Config{
		Port:               "8080",
		BindAddress:        "0.0.0.0",
		ServerName:         DefaultServerName,
		PublicHost:         "",
		CapacityGbps:       25,
		MaxTestDuration:    300 * time.Second,
		ReadTimeout:        0,                // disabled; upload handlers manage own body deadline
		ReadHeaderTimeout:  15 * time.Second, // protects against slowloris
		WriteTimeout:       0,                // disabled; streaming endpoints manage own duration
		IdleTimeout:        60 * time.Second,
		PprofEnabled:       false,
		PprofAddress:       "127.0.0.1:6060",
		PerfStatsInterval:  0,
		RuntimeMetrics:     false,
		RateLimitPerIP:     100,
		MaxConcurrentPerIP: 10,
		GlobalRateLimit:    1000,
		TrustProxyHeaders:  false,
		TrustedProxyCIDRs:  nil,
		AllowedOrigins:     []string{"*"},
		WebRoot:            "",
		DataDir:            "./data",
		MaxStoredResults:   10000,
		TLSCertFile:        "",
		TLSKeyFile:         "",
		TLSAutoGen:         true, // Auto-generate for dev by default
	}
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
	c.loadTLSEnv()
	return nil
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
	if err := c.validateTLS(); err != nil {
		return err
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
