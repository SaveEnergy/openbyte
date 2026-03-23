package stream

import (
	"net"
	"testing"

	"github.com/saveenergy/openbyte/internal/config"
)

// BenchmarkUDPSendDownloadPacket exercises udpSendDownloadPacket (UDP download sender hot path).
func BenchmarkUDPSendDownloadPacket(b *testing.B) {
	cfg := config.DefaultConfig()
	if cfg.UDPBufferSize <= 0 {
		cfg.UDPBufferSize = 1400
	}

	serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		b.Fatal(err)
	}
	defer serverConn.Close()

	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		b.Fatal(err)
	}
	defer clientConn.Close()

	clientAddr := clientConn.LocalAddr().(*net.UDPAddr)
	go func() {
		buf := make([]byte, 65536)
		for {
			if _, _, err := clientConn.ReadFromUDP(buf); err != nil {
				return
			}
		}
	}()

	randomData := make([]byte, 64*1024)
	s := &Server{
		config:     cfg,
		udpConn:    serverConn,
		randomData: randomData,
	}
	packet := make([]byte, cfg.UDPBufferSize)
	n := min(len(packet), len(randomData))
	copy(packet, randomData[:n])
	cli := &udpClientState{
		addr:        clientAddr,
		downloading: 1,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if udpSendDownloadPacket(s, packet, cli) != nil {
			b.Fatal("udp send failed")
		}
	}
}
