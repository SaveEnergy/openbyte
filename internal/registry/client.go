package registry

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
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
