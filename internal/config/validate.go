package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

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
