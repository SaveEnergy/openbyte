package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

const bearerTokenPrefix = "Bearer "

const (
	defaultRegistryInterval   = 30 * time.Second
	maxHeartbeatBackoffFactor = 8
	heartbeatJitterDivisor    = 5 // 20% jitter
	minimumJitterWindow       = 5 * time.Millisecond
)

type Client struct {
	config     *config.Config
	httpClient *http.Client
	logger     *logging.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
	mu         sync.RWMutex
	started    bool
	registered bool
	rngMu      sync.Mutex
	rng        *rand.Rand
}

func NewClient(cfg *config.Config, logger *logging.Logger) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger: logger,
		stopCh: make(chan struct{}),
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
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

	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.mu.Unlock()

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

	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.wg.Wait()

		c.mu.RLock()
		registered := c.registered
		c.mu.RUnlock()
		if registered {
			c.deregister()
		}
	})
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

func (c *Client) heartbeatLoop(getActiveTests func() int) {
	defer c.wg.Done()

	baseInterval := c.config.RegistryInterval
	if baseInterval <= 0 {
		baseInterval = defaultRegistryInterval
	}
	timer := time.NewTimer(baseInterval)
	defer timer.Stop()
	failureCount := 0

	for {
		select {
		case <-c.stopCh:
			return
		case <-timer.C:
			if err := c.heartbeat(getActiveTests()); err != nil {
				failureCount++
				delay := c.nextHeartbeatDelay(baseInterval, failureCount)
				c.logger.Error("Registry heartbeat failed",
					logging.Field{Key: "error", Value: err},
					logging.Field{Key: "retry_in_ms", Value: delay.Milliseconds()})
				timer.Reset(delay)
				continue
			}
			failureCount = 0
			timer.Reset(c.addJitter(baseInterval))
		}
	}
}

func (c *Client) nextHeartbeatDelay(baseInterval time.Duration, failures int) time.Duration {
	backoff := baseInterval
	for i := 1; i < failures && i < maxHeartbeatBackoffFactor; i++ {
		backoff *= 2
	}
	maxBackoff := baseInterval * maxHeartbeatBackoffFactor
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	return c.addJitter(backoff)
}

func (c *Client) addJitter(base time.Duration) time.Duration {
	if base <= minimumJitterWindow {
		return base
	}
	jitterWindow := base / heartbeatJitterDivisor
	if jitterWindow <= 0 {
		return base
	}
	max := int64(jitterWindow*2 + 1)
	offset := c.randomInt63n(max) - int64(jitterWindow)
	jittered := base + time.Duration(offset)
	if jittered <= 0 {
		return base
	}
	return jittered
}

func (c *Client) randomInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	c.rngMu.Lock()
	v := c.rng.Int63n(n)
	c.rngMu.Unlock()
	return v
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
