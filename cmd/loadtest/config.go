package main

import (
	"flag"
	"fmt"
	"net/url"
	"time"
)

type config struct {
	mode        string
	host        string
	tcpPort     int
	udpPort     int
	duration    time.Duration
	concurrency int
	packetSize  int
	wsURL       string
}

const (
	modeTCPDownload      = "tcp-download"
	modeTCPUpload        = "tcp-upload"
	modeTCPBidirectional = "tcp-bidirectional"
	modeUDPDownload      = "udp-download"
	modeUDPUpload        = "udp-upload"
	modeWS               = "ws"
	defaultHost          = "127.0.0.1"
)

var validModes = map[string]struct{}{
	modeTCPDownload:      {},
	modeTCPUpload:        {},
	modeTCPBidirectional: {},
	modeUDPDownload:      {},
	modeUDPUpload:        {},
	modeWS:               {},
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", modeTCPDownload, "Mode: tcp-download, tcp-upload, tcp-bidirectional, udp-download, udp-upload, ws")
	flag.StringVar(&cfg.host, "host", defaultHost, "Target host for TCP/UDP")
	flag.IntVar(&cfg.tcpPort, "tcp-port", 8081, "TCP test port")
	flag.IntVar(&cfg.udpPort, "udp-port", 8082, "UDP test port")
	flag.DurationVar(&cfg.duration, "duration", 10*time.Second, "Test duration (e.g. 10s)")
	flag.IntVar(&cfg.concurrency, "concurrency", 1, "Concurrent workers")
	flag.IntVar(&cfg.packetSize, "packet-size", 1200, "UDP packet size in bytes")
	flag.StringVar(&cfg.wsURL, "ws-url", "", "WebSocket URL for ws mode")
	flag.Parse()
	return cfg
}

func validateConfig(cfg config) error {
	if cfg.concurrency <= 0 {
		return fmt.Errorf("concurrency must be > 0")
	}
	if cfg.duration <= 0 {
		return fmt.Errorf("duration must be > 0")
	}
	if _, ok := validModes[cfg.mode]; !ok {
		return fmt.Errorf("invalid mode: %s", cfg.mode)
	}
	if cfg.mode == modeWS && cfg.wsURL == "" {
		return fmt.Errorf("ws-url required for ws mode")
	}
	if err := validatePortRange("tcp-port", cfg.tcpPort); err != nil {
		return err
	}
	if err := validatePortRange("udp-port", cfg.udpPort); err != nil {
		return err
	}
	return validatePacketAndWebsocket(cfg)
}

func validatePortRange(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be 1-65535", name)
	}
	return nil
}

func validatePacketAndWebsocket(cfg config) error {
	if cfg.packetSize < 64 {
		return fmt.Errorf("packet-size must be >= 64")
	}
	if cfg.packetSize > 9000 {
		return fmt.Errorf("packet-size must be <= 9000")
	}
	return validateWebSocketConfig(cfg)
}

func validateWebSocketConfig(cfg config) error {
	if cfg.mode != modeWS {
		return nil
	}
	parsed, err := url.Parse(cfg.wsURL)
	if err != nil {
		return fmt.Errorf("invalid ws-url: %w", err)
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return fmt.Errorf("ws-url scheme must be ws or wss")
	}
	if parsed.Host == "" {
		return fmt.Errorf("ws-url host is required")
	}
	return nil
}
