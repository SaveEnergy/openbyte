package types

import (
	"net"
	"net/url"
	"strings"
)

type NetworkInfo struct {
	ClientIP    string `json:"client_ip"`
	ServerIP    string `json:"server_ip"`
	ISP         string `json:"isp,omitempty"`
	ASN         int    `json:"asn,omitempty"`
	IPv6        bool   `json:"ipv6"`
	NATDetected bool   `json:"nat_detected"`
	MTU         int    `json:"mtu"`
	Hostname    string `json:"hostname,omitempty"`
}

func NewNetworkInfo() *NetworkInfo {
	return &NetworkInfo{
		MTU: 1500,
	}
}

func (n *NetworkInfo) SetClientIP(ip string) {
	n.ClientIP = sanitizeIP(ip)
	n.IPv6 = strings.Contains(n.ClientIP, ":")
}

func (n *NetworkInfo) SetServerIP(ip string) {
	n.ServerIP = sanitizeIP(ip)
}

func (n *NetworkInfo) DetectNAT(localIP, remoteSeenIP string) {
	local := sanitizeIP(localIP)
	remote := sanitizeIP(remoteSeenIP)
	n.NATDetected = local != remote && !isPrivateIP(remote)
}

func sanitizeIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

var privateNets []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
		"fe80::/10",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil || network == nil {
			continue
		}
		privateNets = append(privateNets, network)
	}
}

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range privateNets {
		if network == nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String()
		}
	}
	return ""
}

func GetDefaultInterface() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		return iface.Name
	}
	return ""
}

func DetectMTU(ifaceName string) int {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return 1500
	}
	return iface.MTU
}

// StripHostPort removes the port from a host string, handling IPv6 brackets.
func StripHostPort(host string) string {
	if host == "" {
		return host
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	return host
}

// OriginHost extracts the hostname from an origin URL string.
func OriginHost(origin string) string {
	parsed, err := url.Parse(origin)
	if err == nil && parsed.Host != "" {
		return StripHostPort(parsed.Host)
	}
	return StripHostPort(origin)
}
