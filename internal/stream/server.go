package stream

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync"
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
	wg               sync.WaitGroup
	stopCh           chan struct{}
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
		stopCh:        make(chan struct{}),
		randomData:    make([]byte, randomDataSize),
		recvPool: sync.Pool{
			New: func() any {
				return newRecvBuffer()
			},
		},
	}

	if _, err := rand.Read(s.randomData); err != nil {
		return nil, fmt.Errorf("generate random data: %w", err)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", cfg.GetTCPTestAddress())
	if err != nil {
		return nil, fmt.Errorf("resolve TCP address: %w", err)
	}

	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen TCP: %w", err)
	}
	s.tcpListener = tcpListener

	udpAddr, err := net.ResolveUDPAddr("udp", cfg.GetUDPTestAddress())
	if err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		tcpListener.Close()
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

func newRecvBuffer() *[]byte {
	buf := make([]byte, recvBufferSize)
	return &buf
}

func (s *Server) getRecvBuffer() *[]byte {
	bufPtr, ok := s.recvPool.Get().(*[]byte)
	if !ok || bufPtr == nil || len(*bufPtr) != recvBufferSize {
		return newRecvBuffer()
	}
	return bufPtr
}

func (s *Server) isStopping() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		close(s.stopCh)

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
