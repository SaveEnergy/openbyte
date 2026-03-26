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

// isUnspecifiedBind reports whether addr is a wildcard listen address that must
// not appear in browser-facing URLs (fetch / WebSocket from the UI).
func isUnspecifiedBind(addr string) bool {
	host := strings.TrimSpace(addr)
	if host == "" {
		return true
	}
	// No port — only literal 0.0.0.0 is unspecified among bind strings without ':'.
	if strings.IndexByte(host, ':') < 0 {
		return host == "0.0.0.0"
	}
	// Bracketed [::] with a trailing port: slice compare only (no SplitHostPort / "["+h+"]" alloc).
	if len(host) > 0 && host[0] == '[' {
		if close := strings.IndexByte(host, ']'); close > 1 && close+1 < len(host) && host[close+1] == ':' {
			if host[:close+1] == "[::]" {
				return true
			}
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		// Use the split host only; do not rebuild bracketed IPv6 (alloc). Switch
		// matches "::" and "0.0.0.0"; "[::]" remains only when addr has no port
		// and SplitHostPort leaves the trimmed string unchanged.
		host = h
	}
	switch host {
	case "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}

func requestScheme(r *http.Request, cfg *config.Config) string {
	if r == nil {
		return "http"
	}
	if cfg != nil && cfg.TrustProxyHeaders {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			if forwardedProtoIsHTTPS(proto) {
				return "https"
			}
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
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

func appendPortIfNonDefault(host, port string) string {
	if port == "" || port == "80" || port == "443" {
		return host
	}
	return host + ":" + port
}

func hostWhenBindUnspecified(r *http.Request, port string) string {
	if r != nil && r.Host != "" {
		return r.Host
	}
	return appendPortIfNonDefault(loopbackIPv4, port)
}

func hostForUntrustedProxy(r *http.Request, cfg *config.Config) string {
	if isUnspecifiedBind(cfg.BindAddress) {
		return hostWhenBindUnspecified(r, cfg.Port)
	}
	h := normalizeHost(cfg.BindAddress)
	return appendPortIfNonDefault(h, cfg.Port)
}

// responseHostForEndpoint returns host or host:port for API endpoint construction.
// When proxied and using r.Host, preserves non-standard ports (e.g. proxy:8443).
func responseHostForEndpoint(r *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if cfg.PublicHost != "" {
			return cfg.PublicHost
		}
		if !cfg.TrustProxyHeaders {
			return hostForUntrustedProxy(r, cfg)
		}
	}
	if r != nil && r.Host != "" {
		return r.Host
	}
	return loopbackIPv4
}
