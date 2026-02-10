package stream

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

type Server struct {
	config           *config.Config
	tcpListener      *net.TCPListener
	udpConn          *net.UDPConn
	activeTCPConns   int64
	maxTCPConns      int64
	activeUDPSenders int64
	maxUDPSenders    int64
	maxConnDur       time.Duration
	mu               sync.RWMutex
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	randomData       []byte
	recvPool         sync.Pool
	closeOnce        sync.Once
}

const (
	sendBufferSize = 256 * 1024
	recvBufferSize = 256 * 1024
	randomDataSize = 1024 * 1024
)

func NewServer(cfg *config.Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	maxTCP := int64(cfg.MaxConcurrentHTTP())
	if maxTCP <= 0 {
		maxTCP = 64
	}
	maxDur := cfg.MaxTestDuration + 30*time.Second
	if maxDur <= 30*time.Second {
		maxDur = 330 * time.Second // 300s test + 30s grace
	}

	s := &Server{
		config:        cfg,
		maxTCPConns:   maxTCP,
		maxUDPSenders: maxTCP,
		maxConnDur:    maxDur,
		ctx:           ctx,
		cancel:        cancel,
		randomData:    make([]byte, randomDataSize),
		recvPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, recvBufferSize)
			},
		},
	}

	if _, err := rand.Read(s.randomData); err != nil {
		cancel()
		return nil, fmt.Errorf("generate random data: %w", err)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", cfg.GetTCPTestAddress())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("resolve TCP address: %w", err)
	}

	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("listen TCP: %w", err)
	}
	s.tcpListener = tcpListener

	udpAddr, err := net.ResolveUDPAddr("udp", cfg.GetUDPTestAddress())
	if err != nil {
		tcpListener.Close()
		cancel()
		return nil, fmt.Errorf("resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		tcpListener.Close()
		cancel()
		return nil, fmt.Errorf("listen UDP: %w", err)
	}
	s.udpConn = udpConn

	s.wg.Add(2)
	go s.acceptTCP()
	go s.handleUDP()

	logging.Info("Stream server started",
		logging.Field{Key: "tcp", Value: cfg.GetTCPTestAddress()},
		logging.Field{Key: "udp", Value: cfg.GetUDPTestAddress()})

	return s, nil
}

func (s *Server) acceptTCP() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.tcpListener.SetDeadline(time.Now().Add(100 * time.Millisecond))
			conn, err := s.tcpListener.AcceptTCP()
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				if s.ctx.Err() != nil {
					return
				}
				logging.Warn("TCP accept error", logging.Field{Key: "error", Value: err})
				continue
			}

			conn.SetNoDelay(true)
			conn.SetReadBuffer(recvBufferSize)
			conn.SetWriteBuffer(sendBufferSize)

			if v := atomic.AddInt64(&s.activeTCPConns, 1); v > s.maxTCPConns {
				atomic.AddInt64(&s.activeTCPConns, -1)
				conn.Close()
				continue
			}

			s.wg.Add(1)
			go s.handleTCPConnection(conn)
		}
	}
}

func (s *Server) handleTCPConnection(conn *net.TCPConn) {
	defer s.wg.Done()
	defer conn.Close()
	defer atomic.AddInt64(&s.activeTCPConns, -1)

	// Hard duration cap — prevents indefinitely held connections.
	connCtx, connCancel := context.WithTimeout(s.ctx, s.maxConnDur)
	defer connCancel()
	go func() {
		<-connCtx.Done()
		conn.SetDeadline(time.Now())
	}()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	cmd := make([]byte, 1)
	if _, err := conn.Read(cmd); err != nil {
		return
	}
	conn.SetReadDeadline(time.Time{})

	switch cmd[0] {
	case 'D':
		s.handleDownload(conn)
	case 'U':
		s.handleUpload(conn)
	case 'B':
		s.handleBidirectional(conn)
	default:
		s.handleEcho(conn)
	}
}

func (s *Server) handleDownload(conn *net.TCPConn) {
	dataLen := len(s.randomData)
	offset := 0
	chunkSize := sendBufferSize
	if chunkSize > dataLen {
		chunkSize = dataLen
	}
	nextDeadline := time.Now().Add(1 * time.Second)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if time.Now().After(nextDeadline) {
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				nextDeadline = time.Now().Add(1 * time.Second)
			}
			if offset+chunkSize <= dataLen {
				if _, err := conn.Write(s.randomData[offset : offset+chunkSize]); err != nil {
					return
				}
				offset += chunkSize
				if offset == dataLen {
					offset = 0
				}
				continue
			}
			first := s.randomData[offset:]
			if _, err := conn.Write(first); err != nil {
				return
			}
			remaining := chunkSize - len(first)
			if remaining > 0 {
				if _, err := conn.Write(s.randomData[:remaining]); err != nil {
					return
				}
			}
			offset = remaining
			if offset >= dataLen {
				offset = 0
			}
		}
	}
}

func (s *Server) handleUpload(conn *net.TCPConn) {
	buf := s.getRecvBuffer()
	defer s.recvPool.Put(buf)
	nextDeadline := time.Now().Add(1 * time.Second)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if time.Now().After(nextDeadline) {
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				nextDeadline = time.Now().Add(1 * time.Second)
			}
			_, err := conn.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return
			}
		}
	}
}

func (s *Server) handleBidirectional(conn *net.TCPConn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		dataLen := len(s.randomData)
		chunkSize := sendBufferSize
		if chunkSize > dataLen {
			chunkSize = dataLen
		}
		offset := 0
		nextDeadline := time.Now().Add(1 * time.Second)
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				if time.Now().After(nextDeadline) {
					conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					nextDeadline = time.Now().Add(1 * time.Second)
				}
				if offset+chunkSize <= dataLen {
					if _, err := conn.Write(s.randomData[offset : offset+chunkSize]); err != nil {
						return
					}
					offset += chunkSize
					if offset == dataLen {
						offset = 0
					}
					continue
				}
				first := s.randomData[offset:]
				if _, err := conn.Write(first); err != nil {
					return
				}
				remaining := chunkSize - len(first)
				if remaining > 0 {
					if _, err := conn.Write(s.randomData[:remaining]); err != nil {
						return
					}
				}
				offset = remaining
				if offset >= dataLen {
					offset = 0
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := s.getRecvBuffer()
		defer s.recvPool.Put(buf)
		nextDeadline := time.Now().Add(1 * time.Second)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				if time.Now().After(nextDeadline) {
					conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					nextDeadline = time.Now().Add(1 * time.Second)
				}
				_, err := conn.Read(buf)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
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
}

func (s *Server) handleEcho(conn *net.TCPConn) {
	buf := s.getRecvBuffer()
	defer s.recvPool.Put(buf)
	nextDeadline := time.Now().Add(1 * time.Second)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if time.Now().After(nextDeadline) {
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))
				nextDeadline = time.Now().Add(1 * time.Second)
			}
			n, err := conn.Read(buf)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return
			}

			if n > 0 {
				if _, err := conn.Write(buf[:n]); err != nil {
					return
				}
			}
		}
	}
}

// udpClients tracks active UDP client state, safe for concurrent access.
type udpClients struct {
	mu sync.RWMutex
	m  map[string]*udpClientState
}

func (c *udpClients) get(key string) *udpClientState {
	c.mu.RLock()
	client := c.m[key]
	c.mu.RUnlock()
	return client
}

// getOrCreate returns an existing client or creates a new one.
// Returns (nil, false) if the sender limit is reached.
// Returns (client, true) if a new client was created (caller must start sender).
func (c *udpClients) getOrCreate(key string, addr *net.UDPAddr, s *Server) (*udpClientState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.m[key]; existing != nil {
		return existing, false
	}
	if v := atomic.AddInt64(&s.activeUDPSenders, 1); v > s.maxUDPSenders {
		atomic.AddInt64(&s.activeUDPSenders, -1)
		return nil, false
	}
	client := &udpClientState{
		addr:         addr,
		senderActive: 1,
		lastSeenUnix: time.Now().UnixNano(),
	}
	c.m[key] = client
	return client, true
}

func (c *udpClients) cleanup() {
	now := time.Now()
	c.mu.Lock()
	for key, client := range c.m {
		lastSeen := time.Unix(0, atomic.LoadInt64(&client.lastSeenUnix))
		if now.Sub(lastSeen) > 30*time.Second && atomic.LoadInt32(&client.senderActive) == 0 {
			delete(c.m, key)
		}
	}
	c.mu.Unlock()
}

func (c *udpClients) remove(key string) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}

func (s *Server) handleUDP() {
	defer s.wg.Done()

	clients := &udpClients{m: make(map[string]*udpClientState)}

	numReaders := runtime.GOMAXPROCS(0)
	if numReaders < 2 {
		numReaders = 2
	}
	if numReaders > 4 {
		numReaders = 4
	}

	var readersWg sync.WaitGroup
	for i := 0; i < numReaders; i++ {
		readersWg.Add(1)
		go s.udpReader(clients, &readersWg)
	}

	logging.Info("UDP readers started", logging.Field{Key: "count", Value: numReaders})

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			readersWg.Wait()
			return
		case <-ticker.C:
			clients.cleanup()
		}
	}
}

func (s *Server) udpReader(clients *udpClients, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, s.config.UDPBufferSize)

	for {
		_ = s.udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, addr, err := s.udpConn.ReadFromUDP(buf)
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			logging.Warn("UDP read error", logging.Field{Key: "error", Value: err})
			return
		}

		if n == 0 {
			continue
		}

		clientKey := addr.String()
		client := clients.get(clientKey)

		if client == nil {
			var created bool
			client, created = clients.getOrCreate(clientKey, addr, s)
			if client == nil {
				continue
			}
			if created {
				s.wg.Add(1)
				go s.udpSender(clients, clientKey, client)
			}
		}

		atomic.StoreInt64(&client.lastSeenUnix, time.Now().UnixNano())

		switch buf[0] {
		case 'D':
			atomic.StoreInt32(&client.downloading, 1)
		case 'U':
			atomic.AddInt64(&client.bytesRecv, int64(n))
		case 'S':
			atomic.StoreInt32(&client.downloading, 0)
		default:
			if _, err := s.udpConn.WriteToUDP(buf[:n], addr); err != nil {
				logging.Warn("UDP echo error", logging.Field{Key: "error", Value: err})
			}
		}
	}
}

type udpClientState struct {
	addr         *net.UDPAddr
	downloading  int32
	senderActive int32
	bytesRecv    int64
	lastSeenUnix int64
}

func (s *Server) udpSender(clients *udpClients, clientKey string, client *udpClientState) {
	defer s.wg.Done()
	defer atomic.AddInt64(&s.activeUDPSenders, -1)
	defer atomic.StoreInt32(&client.senderActive, 0)
	defer clients.remove(clientKey)
	defer func() {
		if s.udpConn != nil {
			_ = s.udpConn.SetWriteDeadline(time.Time{})
		}
	}()

	packet := make([]byte, s.config.UDPBufferSize)
	n := len(packet)
	if n > len(s.randomData) {
		n = len(s.randomData)
	}
	copy(packet, s.randomData[:n])

	lastYield := time.Now()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			lastSeen := time.Unix(0, atomic.LoadInt64(&client.lastSeenUnix))
			if time.Since(lastSeen) > 30*time.Second {
				return
			}
			if atomic.LoadInt32(&client.downloading) == 1 {
				_ = s.udpConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := s.udpConn.WriteToUDP(packet, client.addr); err != nil {
					logging.Warn("UDP send error", logging.Field{Key: "error", Value: err})
					return
				}
				if time.Since(lastYield) > 2*time.Millisecond {
					runtime.Gosched()
					lastYield = time.Now()
				}
				continue
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *Server) getRecvBuffer() []byte {
	buf, ok := s.recvPool.Get().([]byte)
	if !ok || len(buf) != recvBufferSize {
		return make([]byte, recvBufferSize)
	}
	return buf
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()

		if s.tcpListener != nil {
			s.tcpListener.Close()
		}
		if s.udpConn != nil {
			s.udpConn.Close()
		}

		s.wg.Wait()
	})
	return nil
}
