package config

import (
	"fmt"
	"time"
)

type Config struct {
	Port        string
	BindAddress string

	TCPTestPort int
	UDPTestPort int

	PublicHost   string
	CapacityGbps int

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

	TLSCertFile string
	TLSKeyFile  string
	TLSAutoGen  bool // Auto-generate self-signed cert for dev
}

func DefaultConfig() *Config {
	return &Config{
		Port:                  "8080",
		BindAddress:           "0.0.0.0",
		TCPTestPort:           8081,
		UDPTestPort:           8082,
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
		TLSCertFile:           "",
		TLSKeyFile:            "",
		TLSAutoGen:            true, // Auto-generate for dev by default
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

func (c *Config) GetTCPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.TCPTestPort)
}

func (c *Config) GetUDPTestAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.UDPTestPort)
}
