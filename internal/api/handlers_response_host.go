package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
)

const loopbackIPv4 = "127.0.0.1"

func normalizeHost(host string) string {
	if host == "" {
		return loopbackIPv4
	}
	// No port — avoid net.SplitHostPort when the input cannot be host:port.
	if strings.IndexByte(host, ':') < 0 {
		if host == "localhost" {
			return loopbackIPv4
		}
		return host
	}
	trimmed := host
	// Bracketed IPv6 with a trailing port: reuse the bracketed host substring
	// instead of allocating "[" + h + "]" after SplitHostPort.
	if len(host) > 0 && host[0] == '[' {
		if close := strings.IndexByte(host, ']'); close > 1 && close+1 < len(host) && host[close+1] == ':' {
			return host[:close+1]
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		trimmed = h
		if strings.IndexByte(h, ':') >= 0 && strings.IndexByte(host, '[') >= 0 {
			trimmed = "[" + h + "]"
		}
	}
	if trimmed == "" || trimmed == "localhost" {
		return loopbackIPv4
	}
	return trimmed
}

func responseHost(r *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if cfg.PublicHost != "" {
			return cfg.PublicHost
		}
		if !cfg.TrustProxyHeaders {
			return normalizeHost(cfg.BindAddress)
		}
	}
	return normalizeHost(r.Host)
}
