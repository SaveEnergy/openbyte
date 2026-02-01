package types

import (
	"net"
	"strings"
)

type NetworkInfo struct {
	ClientIP       string `json:"client_ip"`
	ServerIP       string `json:"server_ip"`
	ISP            string `json:"isp,omitempty"`
	ASN            int    `json:"asn,omitempty"`
	IPv6           bool   `json:"ipv6"`
	NATDetected    bool   `json:"nat_detected"`
	MTU            int    `json:"mtu"`
	Hostname       string `json:"hostname,omitempty"`
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

func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
		"fe80::/10",
	}

	for _, block := range privateBlocks {
		_, cidr, err := net.ParseCIDR(block)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
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
