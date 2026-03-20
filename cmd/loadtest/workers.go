package main

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func runLoadtest(ctx context.Context, cfg config) (int64, int64, int64) {
	var bytesSent int64
	var bytesRecv int64
	var workerErrs int64

	var wg sync.WaitGroup
	wg.Add(cfg.concurrency)
	for i := 0; i < cfg.concurrency; i++ {
		go func(worker int) {
			defer wg.Done()
			sent, recv, err := runLoadtestWorker(ctx, cfg, worker)
			atomic.AddInt64(&bytesSent, sent)
			atomic.AddInt64(&bytesRecv, recv)
			if err != nil && !errors.Is(err, context.Canceled) {
				atomic.AddInt64(&workerErrs, 1)
			}
		}(i)
	}
	wg.Wait()
	return atomic.LoadInt64(&bytesSent), atomic.LoadInt64(&bytesRecv), atomic.LoadInt64(&workerErrs)
}

func runLoadtestWorker(ctx context.Context, cfg config, worker int) (int64, int64, error) {
	switch cfg.mode {
	case modeTCPDownload:
		recv, err := runTCPDownload(ctx, cfg, worker)
		return 0, recv, err
	case modeTCPUpload:
		sent, err := runTCPUpload(ctx, cfg, worker)
		return sent, 0, err
	case modeTCPBidirectional:
		return runTCPBidirectional(ctx, cfg, worker)
	case modeUDPDownload:
		recv, err := runUDPDownload(ctx, cfg, worker)
		return 0, recv, err
	case modeUDPUpload:
		sent, err := runUDPUpload(ctx, cfg, worker)
		return sent, 0, err
	case modeWS:
		return 0, 0, runWebSocket(ctx, cfg)
	default:
		return 0, 0, fmt.Errorf("invalid mode: %s", cfg.mode)
	}
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
		runTCPBidirectionalWriteLoop(ctx, conn, writeBuf, &sent)
	}()

	go func() {
		defer wg.Done()
		runTCPBidirectionalReadLoop(ctx, conn, readBuf, &recv)
	}()

	wg.Wait()
	return sent, recv, nil
}

func runTCPBidirectionalWriteLoop(ctx context.Context, conn net.Conn, writeBuf []byte, sent *int64) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Write(writeBuf)
			if n > 0 {
				atomic.AddInt64(sent, int64(n))
			}
			if err != nil {
				return
			}
		}
	}
}

func runTCPBidirectionalReadLoop(ctx context.Context, conn net.Conn, readBuf []byte, recv *int64) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(readBuf)
			if n > 0 {
				atomic.AddInt64(recv, int64(n))
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

func runWebSocket(ctx context.Context, cfg config) error {
	parsed, err := url.Parse(cfg.wsURL)
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: parsed.Hostname(),
		},
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
	if _, err := cryptorand.Read(buf); err == nil {
		return
	}
	for i := range buf {
		buf[i] = byte((i + seed) % 251)
	}
}
