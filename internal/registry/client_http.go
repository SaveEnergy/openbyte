package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

func (c *Client) register(activeTests int) error {
	info := c.buildServerInfo(activeTests)

	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal server info: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.RegistryURL+"/api/v1/registry/servers", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.RegistryAPIKey != "" {
		req.Header.Set("Authorization", bearerTokenPrefix+c.config.RegistryAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer drainAndClose(resp, c.logger)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	c.mu.Lock()
	c.registered = true
	c.mu.Unlock()

	c.logger.Info("Registered with registry")
	return nil
}

func (c *Client) heartbeat(activeTests int) error {
	info := c.buildServerInfo(activeTests)

	payload, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal server info: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/registry/servers/%s", c.config.RegistryURL, c.config.ServerID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.RegistryAPIKey != "" {
		req.Header.Set("Authorization", bearerTokenPrefix+c.config.RegistryAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer drainAndClose(resp, c.logger)

	if resp.StatusCode == http.StatusNotFound {
		return c.register(activeTests)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) deregister() {
	url := fmt.Sprintf("%s/api/v1/registry/servers/%s", c.config.RegistryURL, c.config.ServerID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		c.logger.Error("Create deregister request",
			logging.Field{Key: "error", Value: err})
		return
	}

	if c.config.RegistryAPIKey != "" {
		req.Header.Set("Authorization", bearerTokenPrefix+c.config.RegistryAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Deregister from registry",
			logging.Field{Key: "error", Value: err})
		return
	}
	drainAndClose(resp, c.logger)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		c.logger.Error("Deregister from registry",
			logging.Field{Key: "status_code", Value: resp.StatusCode})
		return
	}
	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()

	c.logger.Info("Deregistered from registry")
}

func drainAndClose(resp *http.Response, logger *logging.Logger) {
	if resp == nil || resp.Body == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	if err := resp.Body.Close(); err != nil && logger != nil {
		logger.Warn("Registry response close failed")
	}
}
