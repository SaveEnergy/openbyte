package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func (c *Config) loadCoreEnv() error {
	if err := c.loadCoreEnvPortsAndBind(); err != nil {
		return err
	}
	if name := strings.TrimSpace(os.Getenv("SERVER_NAME")); name != "" {
		c.ServerName = name
	}
	if durRaw := os.Getenv("MAX_TEST_DURATION"); durRaw != "" {
		d, err := time.ParseDuration(durRaw)
		if err != nil || d < time.Second || d%time.Second != 0 {
			return fmt.Errorf("invalid MAX_TEST_DURATION %q: must be a whole number of seconds >= 1s (e.g. 300s)", durRaw)
		}
		c.MaxTestDuration = d
	}
	return nil
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
	return nil
}
