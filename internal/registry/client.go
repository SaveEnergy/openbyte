package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

type Client struct {
	config     *config.Config
	httpClient *http.Client
	logger     *logging.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex
	registered bool
}

func NewClient(cfg *config.Config, logger *logging.Logger) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (c *Client) Start(getActiveTests func() int) error {
	if !c.config.RegistryEnabled {
		return nil
	}

	if c.config.RegistryURL == "" {
		c.logger.Warn("Registry enabled but no URL configured")
		return nil
	}

	c.logger.Info("Starting registry client")

	if err := c.register(getActiveTests()); err != nil {
		c.logger.Error("Initial registration failed",
			logging.Field{Key: "error", Value: err})
	}

	c.wg.Add(1)
	go c.heartbeatLoop(getActiveTests)

	return nil
}

func (c *Client) Stop() {
	if !c.config.RegistryEnabled {
		return
	}

	close(c.stopCh)
	c.wg.Wait()

	c.mu.RLock()
	registered := c.registered
	c.mu.RUnlock()
	if registered {
		c.deregister()
	}
}

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
		req.Header.Set("Authorization", "Bearer "+c.config.RegistryAPIKey)
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
		req.Header.Set("Authorization", "Bearer "+c.config.RegistryAPIKey)
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
		req.Header.Set("Authorization", "Bearer "+c.config.RegistryAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Deregister from registry",
			logging.Field{Key: "error", Value: err})
		return
	}
	drainAndClose(resp, c.logger)

	c.logger.Info("Deregistered from registry")
}

func (c *Client) heartbeatLoop(getActiveTests func() int) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.RegistryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.heartbeat(getActiveTests()); err != nil {
				c.logger.Error("Registry heartbeat failed",
					logging.Field{Key: "error", Value: err})
			}
		}
	}
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

func (c *Client) buildServerInfo(activeTests int) types.ServerInfo {
	host := c.config.PublicHost
	if host == "" {
		host = c.config.BindAddress
		if host == "0.0.0.0" {
			host = "localhost"
		}
	}

	scheme := "http"
	if c.config.TrustProxyHeaders {
		scheme = "https"
	}
	apiEndpoint := fmt.Sprintf("%s://%s:%s", scheme, host, c.config.Port)

	return types.ServerInfo{
		ID:           c.config.ServerID,
		Name:         c.config.ServerName,
		Location:     c.config.ServerLocation,
		Region:       c.config.ServerRegion,
		Host:         host,
		TCPPort:      c.config.TCPTestPort,
		UDPPort:      c.config.UDPTestPort,
		APIEndpoint:  apiEndpoint,
		Health:       "healthy",
		CapacityGbps: c.config.CapacityGbps,
		ActiveTests:  activeTests,
		MaxTests:     c.config.MaxConcurrentTests,
	}
}
