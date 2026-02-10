package main

import (
	"context"
	"net"
	"testing"
	"time"

	icfg "github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestValidateConfig(t *testing.T) {
	valid := config{
		mode:        "tcp-download",
		host:        "127.0.0.1",
		tcpPort:     8081,
		udpPort:     8082,
		duration:    1 * time.Second,
		concurrency: 1,
		packetSize:  1200,
	}

	tests := []struct {
		name    string
		cfg     config
		wantErr bool
	}{
		{name: "valid tcp download", cfg: valid, wantErr: false},
		{name: "invalid mode", cfg: func() config { c := valid; c.mode = "bad"; return c }(), wantErr: true},
		{name: "non-positive concurrency", cfg: func() config { c := valid; c.concurrency = 0; return c }(), wantErr: true},
		{name: "non-positive duration", cfg: func() config { c := valid; c.duration = 0; return c }(), wantErr: true},
		{name: "ws missing url", cfg: func() config { c := valid; c.mode = "ws"; c.wsURL = ""; return c }(), wantErr: true},
		{name: "ws with url", cfg: func() config { c := valid; c.mode = "ws"; c.wsURL = "ws://example.com"; return c }(), wantErr: false},
		{name: "too small packet", cfg: func() config { c := valid; c.packetSize = 63; return c }(), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunTCPDownloadTransfersBytes(t *testing.T) {
	tcpPort := reserveTCPPort(t)
	udpPort := reserveUDPPort(t)
	srv := startTestStreamServer(t, tcpPort, udpPort)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	cfg := config{
		host:    "127.0.0.1",
		tcpPort: tcpPort,
	}
	n, err := runTCPDownload(ctx, cfg, 0)
	if err != nil {
		t.Fatalf("runTCPDownload error: %v", err)
	}
	if n <= 0 {
		t.Fatalf("runTCPDownload bytes = %d, want > 0", n)
	}
}

func TestRunUDPDownloadTransfersBytes(t *testing.T) {
	tcpPort := reserveTCPPort(t)
	udpPort := reserveUDPPort(t)
	srv := startTestStreamServer(t, tcpPort, udpPort)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	cfg := config{
		host:       "127.0.0.1",
		udpPort:    udpPort,
		packetSize: 1200,
	}
	n, err := runUDPDownload(ctx, cfg, 0)
	if err != nil {
		t.Fatalf("runUDPDownload error: %v", err)
	}
	if n <= 0 {
		t.Fatalf("runUDPDownload bytes = %d, want > 0", n)
	}
}

func startTestStreamServer(t *testing.T, tcpPort, udpPort int) *stream.Server {
	t.Helper()
	serverCfg := icfg.DefaultConfig()
	serverCfg.BindAddress = "127.0.0.1"
	serverCfg.TCPTestPort = tcpPort
	serverCfg.UDPTestPort = udpPort

	srv, err := stream.NewServer(serverCfg)
	if err != nil {
		t.Fatalf("new stream server: %v", err)
	}
	return srv
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func reserveUDPPort(t *testing.T) int {
	t.Helper()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("reserve udp port: %v", err)
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port
}
