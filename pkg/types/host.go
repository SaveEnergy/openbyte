package types

import (
	"net"
	"net/url"
	"strings"
)

// StripHostPort removes the port from a host string, handling IPv6 brackets.
func StripHostPort(host string) string {
	if host == "" {
		return host
	}
	// Bracketed hosts first: preserves the SplitHostPort-first behavior for "[ipv6]:port"
	// without an extra scan on the common hostname:port fast path below.
	if host[0] == '[' {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		if n := len(host); n >= 2 && host[n-1] == ']' {
			return host[1 : n-1]
		}
		return host
	}
	// Fast path: single ':' with a decimal port 0–65535 (hostname or IPv4 literal). Skips
	// net.SplitHostPort on typical browser authority values (e.g. app.example.com:8443).
	// Multi-colon hosts (unbracketed IPv6) fall through to SplitHostPort.
	if i := strings.IndexByte(host, ':'); i > 0 {
		if strings.IndexByte(host[i+1:], ':') < 0 && numericDecimalPortOK(host[i+1:]) {
			return host[:i]
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func numericDecimalPortOK(s string) bool {
	if len(s) == 0 || len(s) > 5 {
		return false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
		if n > 65535 {
			return false
		}
	}
	return true
}

// DotBoundarySuffix reports whether host ends with "."+suffix (dot-separated
// suffix). For empty suffix, it reports whether host ends with '.'.
func DotBoundarySuffix(host, suffix string) bool {
	if suffix == "" {
		return len(host) > 0 && host[len(host)-1] == '.'
	}
	if len(host) <= len(suffix) {
		return false
	}
	if host[len(host)-len(suffix)-1] != '.' {
		return false
	}
	return host[len(host)-len(suffix):] == suffix
}

// OriginHost extracts the hostname from an origin URL string.
func OriginHost(origin string) string {
	// Fast path: typical browser origins are absolute URLs with no userinfo.
	// Avoid url.Parse allocation when we can slice scheme://authority[/...] safely.
	if idx := strings.Index(origin, "://"); idx >= 0 {
		rest := origin[idx+3:]
		end := strings.IndexByte(rest, '/')
		var authority string
		if end < 0 {
			authority = rest
		} else {
			authority = rest[:end]
		}
		if authority != "" && strings.IndexByte(authority, '@') < 0 {
			return StripHostPort(authority)
		}
	}
	parsed, err := url.Parse(origin)
	if err == nil && parsed.Host != "" {
		return StripHostPort(parsed.Host)
	}
	return StripHostPort(origin)
}
