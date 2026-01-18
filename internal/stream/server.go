package stream

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

type Server struct {
	config        *config.Config
	tcpListener   *net.TCPListener
	udpConn       *net.UDPConn
	activeStreams map[string]*StreamSession
	mu            sync.RWMutex
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	randomData    []byte
	sendPool      sync.Pool
	recvPool      sync.Pool
}

type StreamSession struct {
	StreamID    string
	Config      types.StreamConfig
	Connections []net.Conn
	StartTime   time.Time
	mu          sync.RWMutex
}

const (
	sendBufferSize = 256 * 1024
	recvBufferSize = 256 * 1024
	randomDataSize = 1024 * 1024
)

func NewServer(cfg *config.Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:        cfg,
		activeStreams: make(map[string]*StreamSession),
		ctx:           ctx,
		cancel:        cancel,
		randomData:    make([]byte, randomDataSize),
		sendPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, sendBufferSize)
			},
		},
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
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
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

			s.wg.Add(1)
			go s.handleTCPConnection(conn)
		}
	}
}

func (s *Server) handleTCPConnection(conn *net.TCPConn) {
	defer s.wg.Done()
	defer conn.Close()

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
	buf := s.recvPool.Get().([]byte)
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
				if err == io.EOF {
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
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
		buf := s.sendPool.Get().([]byte)
		defer s.sendPool.Put(buf)
		dataLen := len(s.randomData)
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
				if offset+len(buf) <= dataLen {
					if _, err := conn.Write(s.randomData[offset : offset+len(buf)]); err != nil {
						return
					}
					offset += len(buf)
					if offset == dataLen {
						offset = 0
					}
					continue
				}
				first := s.randomData[offset:]
				if _, err := conn.Write(first); err != nil {
					return
				}
				remaining := len(buf) - len(first)
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
		buf := s.recvPool.Get().([]byte)
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
					return
				}
			}
		}
	}()

	wg.Wait()
}

func (s *Server) handleEcho(conn *net.TCPConn) {
	buf := s.recvPool.Get().([]byte)
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
				if err == io.EOF {
					return
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			if n > 0 {
				conn.Write(buf[:n])
			}
		}
	}
}

func (s *Server) handleUDP() {
	defer s.wg.Done()

	buf := make([]byte, s.config.UDPBufferSize)
	clients := make(map[string]*udpClientState)
	var clientsMu sync.RWMutex

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, addr, err := s.udpConn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if s.ctx.Err() != nil {
					return
				}
				continue
			}

			if n == 0 {
				continue
			}

			clientKey := addr.String()
			clientsMu.RLock()
			client := clients[clientKey]
			clientsMu.RUnlock()

			if client == nil {
				clientsMu.Lock()
				client = &udpClientState{
					addr:     addr,
					lastSeen: time.Now(),
				}
				clients[clientKey] = client
				clientsMu.Unlock()

				go s.udpSender(client)
			}

			client.lastSeen = time.Now()

			cmd := buf[0]
			switch cmd {
			case 'D':
				atomic.StoreInt32(&client.downloading, 1)
			case 'U':
				atomic.AddInt64(&client.bytesRecv, int64(n))
			case 'S':
				atomic.StoreInt32(&client.downloading, 0)
			default:
				s.udpConn.WriteToUDP(buf[:n], addr)
			}
		}
	}
}

type udpClientState struct {
	addr        *net.UDPAddr
	downloading int32
	bytesRecv   int64
	lastSeen    time.Time
}

func (s *Server) udpSender(client *udpClientState) {
	packet := make([]byte, s.config.UDPBufferSize)
	copy(packet, s.randomData[:len(packet)])

	lastYield := time.Now()
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if time.Since(client.lastSeen) > 30*time.Second {
				return
			}
			if atomic.LoadInt32(&client.downloading) == 1 {
				_, _ = s.udpConn.WriteToUDP(packet, client.addr)
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

func (s *Server) Close() error {
	s.cancel()

	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
	if s.udpConn != nil {
		s.udpConn.Close()
	}

	s.wg.Wait()
	return nil
}
