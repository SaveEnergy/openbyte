package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
)

type ClientIPResolver struct {
	trustProxyHeaders bool
	trustedProxyNets  []*net.IPNet
}

func NewClientIPResolver(cfg *config.Config) *ClientIPResolver {
	if cfg == nil {
		return &ClientIPResolver{}
	}
	return &ClientIPResolver{
		trustProxyHeaders: cfg.TrustProxyHeaders,
		trustedProxyNets:  parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs),
	}
}

func (r *ClientIPResolver) FromRequest(req *http.Request) string {
	remoteIP := parseRemoteIP(req.RemoteAddr)
	if !r.trustProxyHeaders || !r.isTrustedProxy(remoteIP) {
		return ipString(remoteIP)
	}

	if clientIP := r.rightmostUntrustedIP(req.Header.Get("X-Forwarded-For")); clientIP != nil {
		return ipString(clientIP)
	}
	if clientIP := parseHeaderIP(req.Header.Get("X-Real-IP")); clientIP != nil {
		return ipString(clientIP)
	}

	return ipString(remoteIP)
}

// rightmostUntrustedIP walks X-Forwarded-For entries from right to left,
// skipping trusted proxy IPs. The first non-trusted entry is the real client.
// This prevents spoofing via attacker-prepended XFF values.
func (r *ClientIPResolver) rightmostUntrustedIP(xff string) net.IP {
	if xff == "" {
		return nil
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := parseHeaderIP(parts[i])
		if ip == nil {
			continue
		}
		if r.isTrustedProxy(ip) {
			continue
		}
		return ip
	}
	return nil
}

func (r *ClientIPResolver) isTrustedProxy(ip net.IP) bool {
	if ip == nil || len(r.trustedProxyNets) == 0 {
		return false
	}
	for _, network := range r.trustedProxyNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseTrustedProxyCIDRs(cidrs []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, entry := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(entry))
		if err == nil && network != nil {
			networks = append(networks, network)
		}
	}
	return networks
}

func parseRemoteIP(remoteAddr string) net.IP {
	if remoteAddr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return parseHeaderIP(host)
	}
	return parseHeaderIP(remoteAddr)
}

func parseHeaderIP(value string) net.IP {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return nil
	}
	if strings.HasPrefix(clean, "[") && strings.Contains(clean, "]") {
		clean = strings.TrimPrefix(clean, "[")
		clean = strings.TrimSuffix(clean, "]")
	}
	if ip := net.ParseIP(clean); ip != nil {
		return ip
	}
	if host, _, err := net.SplitHostPort(clean); err == nil {
		return net.ParseIP(host)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func ipString(ip net.IP) string {
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}
