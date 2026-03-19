package registry

import (
	"fmt"

	"github.com/saveenergy/openbyte/pkg/types"
)

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
