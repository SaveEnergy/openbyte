package stream

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
)

const (
	loopbackAddrWithPort = "127.0.0.1:12345"
	loopbackIPv4         = "127.0.0.1"
	udpAddrA             = "127.0.0.1"
	udpAddrB             = "127.0.0.1"
	udpPortA             = 10001
	udpPortB             = 10002
	readTimeoutShort     = 2 * time.Second
	closeTimeoutLong     = 3 * time.Second
	networkTCP           = "tcp"
	newServerFmt         = "new server: %v"
)

func TestUDPSenderRemovesClientAndDecrementsCountOnExit(t *testing.T) {
	s := &Server{
		stopCh: make(chan struct{}),
		config: &config.Config{UDPBufferSize: 1400},
	}

	clientKey := loopbackAddrWithPort
	client := &udpClientState{
		senderActive: 1,
		// Force immediate sender exit path.
		lastSeenUnix: time.Now().Add(-1 * time.Hour).UnixNano(),
	}
	clients := &udpClients{m: map[string]*udpClientState{
		clientKey: client,
	}}

	atomic.StoreInt64(&s.activeUDPSenders, 1)
	s.wg.Add(1)
	go s.udpSender(clients, clientKey, client)
	s.wg.Wait()

	if got := atomic.LoadInt64(&s.activeUDPSenders); got != 0 {
		t.Fatalf("activeUDPSenders = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&client.senderActive); got != 0 {
		t.Fatalf("client senderActive = %d, want 0", got)
	}
	if clients.get(clientKey) != nil {
		t.Fatalf("client %q should be removed from map on sender exit", clientKey)
	}
}

func TestGetOrCreateRejectsWhenSenderLimitReached(t *testing.T) {
	s := &Server{
		maxUDPSenders: 1,
	}
	clients := &udpClients{m: make(map[string]*udpClientState)}

	addrA := &net.UDPAddr{IP: net.ParseIP(udpAddrA), Port: udpPortA}
	addrB := &net.UDPAddr{IP: net.ParseIP(udpAddrB), Port: udpPortB}

	first, created := clients.getOrCreate("a", addrA, s)
	if first == nil || !created {
		t.Fatal("first client should be created")
	}
	if got := atomic.LoadInt64(&s.activeUDPSenders); got != 1 {
		t.Fatalf("activeUDPSenders after first create = %d, want 1", got)
	}

	second, created := clients.getOrCreate("b", addrB, s)
	if second != nil || created {
		t.Fatal("second client should be rejected when sender limit reached")
	}
	if got := atomic.LoadInt64(&s.activeUDPSenders); got != 1 {
		t.Fatalf("activeUDPSenders after rejected create = %d, want 1", got)
	}
}

func TestAcceptTCPRejectsWhenAtLimit(t *testing.T) {
	addr, err := net.ResolveTCPAddr(networkTCP, loopbackIPv4+":0")
	if err != nil {
		t.Fatalf("resolve tcp addr: %v", err)
	}
	listener, err := net.ListenTCP(networkTCP, addr)
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	srv := &Server{
		tcpListener: listener,
		stopCh:      make(chan struct{}),
		maxTCPConns: 0,
	}
	srv.wg.Add(1)
	go srv.acceptTCP()
	defer srv.Close()

	conn, err := net.DialTimeout(networkTCP, listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(readTimeoutShort))
	_, readErr := conn.Read(buf)
	if readErr == nil {
		t.Fatal("expected connection close/reject when TCP limit is reached")
	}
	var netErr net.Error
	if errors.As(readErr, &netErr) && netErr.Timeout() {
		t.Fatalf("read timed out; expected reject/close error, got timeout: %v", readErr)
	}
}

func TestServerCloseWithActiveUDPReturnsPromptly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BindAddress = loopbackIPv4
	cfg.TCPTestPort = 0
	cfg.UDPTestPort = 0

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf(newServerFmt, err)
	}

	udpAddr, ok := srv.udpConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("failed to resolve udp local addr")
	}

	clientConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		_ = srv.Close()
		t.Fatalf("dial udp: %v", err)
	}
	_, _ = clientConn.Write([]byte{'D'})
	_ = clientConn.Close()

	done := make(chan struct{})
	go func() {
		_ = srv.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(closeTimeoutLong):
		t.Fatal("server close timed out with active udp path")
	}
}

func TestServerCloseIdempotentConcurrent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.BindAddress = loopbackIPv4
	cfg.TCPTestPort = 0
	cfg.UDPTestPort = 0

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf(newServerFmt, err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			errCh <- srv.Close()
		}()
	}
	wg.Wait()
	close(errCh)

	for closeErr := range errCh {
		if closeErr != nil {
			t.Fatalf("close error: %v", closeErr)
		}
	}
}
