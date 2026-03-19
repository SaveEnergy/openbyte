package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

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
