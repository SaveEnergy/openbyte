package config

import "time"

const DefaultServerName = "openByte Server"

type Config struct {
	Port        string
	BindAddress string
	ServerName  string

	// ImpressumURL activates the /impressum redirect and the footer
	// legal-notice link when set to an absolute http(s) URL.
	ImpressumURL string

	MaxTestDuration time.Duration

	PprofEnabled bool
	PprofAddress string

	RateLimitPerIP         int
	GlobalRateLimit        int
	MaxConcurrentTransfers int
	MaxConcurrentPerIP     int

	TrustProxyHeaders bool
	TrustedProxyCIDRs []string

	WebRoot          string
	DataDir          string
	MaxStoredResults int

	BrandPrimaryColorDark    string
	BrandPrimaryColorLight   string
	BrandSecondaryColorDark  string
	BrandSecondaryColorLight string
	BrandLogoPath            string
	brandLogoData            []byte
	brandLogoContentType     string

	TLSCertFile string
	TLSKeyFile  string
	TLSAutoGen  bool // Auto-generate a self-signed cert for dev when explicitly enabled.

	HTTP2Enabled bool
}

func DefaultConfig() *Config {
	return &Config{
		Port:                   "8080",
		BindAddress:            "0.0.0.0",
		ServerName:             DefaultServerName,
		MaxTestDuration:        300 * time.Second,
		PprofEnabled:           false,
		PprofAddress:           "127.0.0.1:6060",
		RateLimitPerIP:         100,
		GlobalRateLimit:        1000,
		MaxConcurrentTransfers: 200,
		MaxConcurrentPerIP:     64,
		TrustProxyHeaders:      false,
		TrustedProxyCIDRs:      nil,
		WebRoot:                "",
		DataDir:                "./data",
		MaxStoredResults:       10000,
		TLSCertFile:            "",
		TLSKeyFile:             "",
		TLSAutoGen:             false,
		HTTP2Enabled:           true,
	}
}

func (c *Config) LoadFromEnv() error {
	if err := c.loadCoreEnv(); err != nil {
		return err
	}
	c.loadRuntimeEnv()
	if err := c.loadLimitsAndNetworkEnv(); err != nil {
		return err
	}
	if err := c.loadStorageEnv(); err != nil {
		return err
	}
	c.loadTLSEnv()
	return c.loadBrandingEnv()
}

func (c *Config) Validate() error {
	if err := c.validatePorts(); err != nil {
		return err
	}
	if err := c.validateLimits(); err != nil {
		return err
	}
	if err := c.validateImpressum(); err != nil {
		return err
	}
	if err := c.validateProxyAndStorage(); err != nil {
		return err
	}
	if err := c.validateTLS(); err != nil {
		return err
	}
	return c.validateBranding()
}
