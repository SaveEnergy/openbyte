package types_test

import (
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

const (
	clientAddrV4Port     = "192.168.1.100:12345"
	clientIPV4           = "192.168.1.100"
	clientAddrV6Loopback = "::1"
	serverAddrV4Port     = "10.0.0.1:8080"
	serverIPV4           = "10.0.0.1"
	publicIPFixture      = "203.0.113.1"
	mtuFallback          = 1500
	unknownInterfaceName = "nonexistent-iface-xyz"
)

func TestNetworkInfoSetClientIP(t *testing.T) {
	n := types.NewNetworkInfo()

	n.SetClientIP(clientAddrV4Port)
	if n.ClientIP != clientIPV4 {
		t.Errorf("ClientIP = %q, want %s", n.ClientIP, clientIPV4)
	}
	if n.IPv6 {
		t.Error("IPv6 should be false for v4 address")
	}

	n.SetClientIP(clientAddrV6Loopback)
	if !n.IPv6 {
		t.Error("IPv6 should be true for v6 address")
	}
}

func TestNetworkInfoSetServerIP(t *testing.T) {
	n := types.NewNetworkInfo()
	n.SetServerIP(serverAddrV4Port)
	if n.ServerIP != serverIPV4 {
		t.Errorf("ServerIP = %q, want %s", n.ServerIP, serverIPV4)
	}
}

func TestNetworkInfoDetectNAT(t *testing.T) {
	n := types.NewNetworkInfo()

	// Same IP — no NAT
	n.DetectNAT(publicIPFixture, publicIPFixture)
	if n.NATDetected {
		t.Error("same IP should not detect NAT")
	}

	// Different IPs, remote is public — NAT detected
	n.DetectNAT(clientIPV4, publicIPFixture)
	if !n.NATDetected {
		t.Error("different IPs (private local, public remote) should detect NAT")
	}

	// Different IPs, remote is private — no NAT (both private)
	n.DetectNAT(clientIPV4, serverIPV4)
	if n.NATDetected {
		t.Error("remote private IP should not indicate NAT")
	}
}

func TestNetworkInfoDefaultMTU(t *testing.T) {
	n := types.NewNetworkInfo()
	if n.MTU != mtuFallback {
		t.Errorf("default MTU = %d, want %d", n.MTU, mtuFallback)
	}
}

func TestDetectMTUUnknownInterface(t *testing.T) {
	mtu := types.DetectMTU(unknownInterfaceName)
	if mtu != mtuFallback {
		t.Errorf("unknown iface MTU = %d, want %d fallback", mtu, mtuFallback)
	}
}

func TestGetLocalIP(t *testing.T) {
	ip := types.GetLocalIP()
	if ip == "" {
		t.Log("GetLocalIP returned empty — may be running without network interface")
	}
}

func TestGetDefaultInterface(t *testing.T) {
	iface := types.GetDefaultInterface()
	if iface == "" {
		t.Log("GetDefaultInterface returned empty — may be running without network interface")
	}
}
