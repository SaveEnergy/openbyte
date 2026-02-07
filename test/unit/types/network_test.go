package types_test

import (
	"testing"

	"github.com/saveenergy/openbyte/pkg/types"
)

func TestNetworkInfoSetClientIP(t *testing.T) {
	n := types.NewNetworkInfo()

	n.SetClientIP("192.168.1.100:12345")
	if n.ClientIP != "192.168.1.100" {
		t.Errorf("ClientIP = %q, want 192.168.1.100", n.ClientIP)
	}
	if n.IPv6 {
		t.Error("IPv6 should be false for v4 address")
	}

	n.SetClientIP("::1")
	if !n.IPv6 {
		t.Error("IPv6 should be true for v6 address")
	}
}

func TestNetworkInfoSetServerIP(t *testing.T) {
	n := types.NewNetworkInfo()
	n.SetServerIP("10.0.0.1:8080")
	if n.ServerIP != "10.0.0.1" {
		t.Errorf("ServerIP = %q, want 10.0.0.1", n.ServerIP)
	}
}

func TestNetworkInfoDetectNAT(t *testing.T) {
	n := types.NewNetworkInfo()

	// Same IP — no NAT
	n.DetectNAT("203.0.113.1", "203.0.113.1")
	if n.NATDetected {
		t.Error("same IP should not detect NAT")
	}

	// Different IPs, remote is public — NAT detected
	n.DetectNAT("192.168.1.100", "203.0.113.1")
	if !n.NATDetected {
		t.Error("different IPs (private local, public remote) should detect NAT")
	}

	// Different IPs, remote is private — no NAT (both private)
	n.DetectNAT("192.168.1.100", "10.0.0.1")
	if n.NATDetected {
		t.Error("remote private IP should not indicate NAT")
	}
}

func TestNetworkInfoDefaultMTU(t *testing.T) {
	n := types.NewNetworkInfo()
	if n.MTU != 1500 {
		t.Errorf("default MTU = %d, want 1500", n.MTU)
	}
}

func TestDetectMTUUnknownInterface(t *testing.T) {
	mtu := types.DetectMTU("nonexistent-iface-xyz")
	if mtu != 1500 {
		t.Errorf("unknown iface MTU = %d, want 1500 fallback", mtu)
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
