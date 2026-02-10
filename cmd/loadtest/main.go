package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
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

func main() {
	cfg := parseFlags()
	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "loadtest: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.duration)
	defer cancel()

	var bytesSent int64
	var bytesRecv int64

	var wg sync.WaitGroup
	wg.Add(cfg.concurrency)
	for i := 0; i < cfg.concurrency; i++ {
		go func(worker int) {
			defer wg.Done()
			switch cfg.mode {
			case "tcp-download":
				n, _ := runTCPDownload(ctx, cfg, worker)
				atomic.AddInt64(&bytesRecv, n)
			case "tcp-upload":
				n, _ := runTCPUpload(ctx, cfg, worker)
				atomic.AddInt64(&bytesSent, n)
			case "tcp-bidirectional":
				sent, recv, _ := runTCPBidirectional(ctx, cfg, worker)
				atomic.AddInt64(&bytesSent, sent)
				atomic.AddInt64(&bytesRecv, recv)
			case "udp-download":
				n, _ := runUDPDownload(ctx, cfg, worker)
				atomic.AddInt64(&bytesRecv, n)
			case "udp-upload":
				n, _ := runUDPUpload(ctx, cfg, worker)
				atomic.AddInt64(&bytesSent, n)
			case "ws":
				_ = runWebSocket(ctx, cfg, worker)
			}
		}(i)
	}
	wg.Wait()

	seconds := cfg.duration.Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	fmt.Printf("mode=%s concurrency=%d duration=%s sent_bytes=%d recv_bytes=%d sent_mbps=%.2f recv_mbps=%.2f\n",
		cfg.mode,
		cfg.concurrency,
		cfg.duration,
		atomic.LoadInt64(&bytesSent),
		atomic.LoadInt64(&bytesRecv),
		float64(atomic.LoadInt64(&bytesSent)*8)/seconds/1_000_000,
		float64(atomic.LoadInt64(&bytesRecv)*8)/seconds/1_000_000,
	)
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.mode, "mode", "tcp-download", "Mode: tcp-download, tcp-upload, tcp-bidirectional, udp-download, udp-upload, ws")
	flag.StringVar(&cfg.host, "host", "127.0.0.1", "Target host for TCP/UDP")
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
	switch cfg.mode {
	case "tcp-download", "tcp-upload", "tcp-bidirectional", "udp-download", "udp-upload", "ws":
	default:
		return fmt.Errorf("invalid mode: %s", cfg.mode)
	}
	if cfg.mode == "ws" && cfg.wsURL == "" {
		return fmt.Errorf("ws-url required for ws mode")
	}
	if cfg.packetSize < 64 {
		return fmt.Errorf("packet-size must be >= 64")
	}
	return nil
}

func runTCPDownload(ctx context.Context, cfg config, worker int) (int64, error) {
	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.tcpPort))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{'D'}); err != nil {
		return 0, err
	}

	buf := make([]byte, 256*1024)
	var total int64
	for {
		select {
		case <-ctx.Done():
			return total, nil
		default:
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buf)
			if n > 0 {
				total += int64(n)
			}
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return total, err
			}
		}
	}
}

func runTCPUpload(ctx context.Context, cfg config, worker int) (int64, error) {
	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.tcpPort))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{'U'}); err != nil {
		return 0, err
	}

	buf := make([]byte, 256*1024)
	fillRandom(buf, worker)
	var total int64
	for {
		select {
		case <-ctx.Done():
			return total, nil
		default:
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Write(buf)
			if n > 0 {
				total += int64(n)
			}
			if err != nil {
				return total, err
			}
		}
	}
}

func runTCPBidirectional(ctx context.Context, cfg config, worker int) (int64, int64, error) {
	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.tcpPort))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{'B'}); err != nil {
		return 0, 0, err
	}

	writeBuf := make([]byte, 256*1024)
	readBuf := make([]byte, 256*1024)
	fillRandom(writeBuf, worker)

	var sent int64
	var recv int64
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				n, err := conn.Write(writeBuf)
				if n > 0 {
					atomic.AddInt64(&sent, int64(n))
				}
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, err := conn.Read(readBuf)
				if n > 0 {
					atomic.AddInt64(&recv, int64(n))
				}
				if err != nil {
					var netErr net.Error
					if errors.As(err, &netErr) && netErr.Timeout() {
						continue
					}
					return
				}
			}
		}
	}()

	wg.Wait()
	return sent, recv, nil
}

func runUDPDownload(ctx context.Context, cfg config, worker int) (int64, error) {
	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.udpPort))
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.WriteToUDP([]byte{'D'}, remote); err != nil {
		return 0, err
	}
	buf := make([]byte, cfg.packetSize)
	var total int64
	for {
		select {
		case <-ctx.Done():
			_, _ = conn.WriteToUDP([]byte{'S'}, remote)
			return total, nil
		default:
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if n > 0 {
				total += int64(n)
			}
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return total, err
			}
		}
	}
}

func runUDPUpload(ctx context.Context, cfg config, worker int) (int64, error) {
	addr := net.JoinHostPort(cfg.host, fmt.Sprintf("%d", cfg.udpPort))
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.WriteToUDP([]byte{'U'}, remote); err != nil {
		return 0, err
	}
	buf := make([]byte, cfg.packetSize)
	fillRandom(buf, worker)
	var total int64
	for {
		select {
		case <-ctx.Done():
			_, _ = conn.WriteToUDP([]byte{'S'}, remote)
			return total, nil
		default:
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.WriteToUDP(buf, remote)
			if n > 0 {
				total += int64(n)
			}
			if err != nil {
				return total, err
			}
		}
	}
}

func runWebSocket(ctx context.Context, cfg config, worker int) error {
	parsed, err := url.Parse(cfg.wsURL)
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	}
	conn, _, err := dialer.DialContext(ctx, parsed.String(), nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			if _, _, err := conn.ReadMessage(); err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return err
			}
		}
	}
}

func fillRandom(buf []byte, seed int) {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(seed)))
	for i := range buf {
		buf[i] = byte(r.Intn(256))
	}
}
