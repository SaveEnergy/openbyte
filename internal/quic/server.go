package quic

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
)

// Server handles QUIC transport for high-speed data transfer
type Server struct {
	listener   *quic.Listener
	config     *config.Config
	tlsConfig  *tls.Config
	randomData []byte
	running    atomic.Bool
	wg         sync.WaitGroup
	recvPool   sync.Pool
}

// NewServer creates a new QUIC transport server
func NewServer(cfg *config.Config, tlsConfig *tls.Config) (*Server, error) {
	// Pre-generate random data for downloads (1MB)
	randomData := make([]byte, 1024*1024)
	rand.Read(randomData)

	return &Server{
		config:     cfg,
		tlsConfig:  tlsConfig,
		randomData: randomData,
		recvPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024)
			},
		},
	}, nil
}

// Start begins listening for QUIC connections
func (s *Server) Start() error {
	quicConfig := &quic.Config{
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		EnableDatagrams:       true,
		Allow0RTT:             true,
	}

	addr := s.config.GetQUICAddress()
	listener, err := quic.ListenAddr(addr, s.tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("failed to start QUIC listener: %w", err)
	}

	s.listener = listener
	s.running.Store(true)

	logging.Info("QUIC server started",
		logging.Field{Key: "address", Value: addr})

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for s.running.Load() {
		conn, err := s.listener.Accept(context.Background())
		if err != nil {
			if s.running.Load() {
				logging.Error("Failed to accept QUIC connection",
					logging.Field{Key: "error", Value: err})
			}
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn *quic.Conn) {
	defer s.wg.Done()
	defer conn.CloseWithError(0, "connection closed")

	logging.Debug("New QUIC connection",
		logging.Field{Key: "remote", Value: conn.RemoteAddr().String()})

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return
		}

		s.wg.Add(1)
		go s.handleStream(stream)
	}
}

func (s *Server) handleStream(stream *quic.Stream) {
	defer s.wg.Done()
	defer stream.Close()

	// Read command byte to determine mode
	cmd := make([]byte, 1)
	if _, err := io.ReadFull(stream, cmd); err != nil {
		return
	}

	switch cmd[0] {
	case 'd', 'D': // Download mode
		s.handleDownload(stream)
	case 'u', 'U': // Upload mode
		s.handleUpload(stream)
	case 'p', 'P': // Ping mode
		s.handlePing(stream)
	default:
		logging.Debug("Unknown QUIC command",
			logging.Field{Key: "cmd", Value: cmd[0]})
	}
}

func (s *Server) handleDownload(stream *quic.Stream) {
	// Read duration (4 bytes, seconds)
	durBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, durBuf); err != nil {
		return
	}
	duration := int(durBuf[0])<<24 | int(durBuf[1])<<16 | int(durBuf[2])<<8 | int(durBuf[3])
	if duration <= 0 || duration > 300 {
		duration = 10
	}

	deadline := time.Now().Add(time.Duration(duration) * time.Second)
	var bytesSent int64

	// Use 64KB chunks for high throughput
	chunk := s.randomData[:64*1024]

	for time.Now().Before(deadline) {
		n, err := stream.Write(chunk)
		if err != nil {
			break
		}
		bytesSent += int64(n)
	}

	logging.Debug("QUIC download complete",
		logging.Field{Key: "bytes", Value: bytesSent},
		logging.Field{Key: "mbps", Value: float64(bytesSent*8) / float64(duration) / 1_000_000})
}

func (s *Server) handleUpload(stream *quic.Stream) {
	// Read duration (4 bytes, seconds)
	durBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, durBuf); err != nil {
		return
	}
	duration := int(durBuf[0])<<24 | int(durBuf[1])<<16 | int(durBuf[2])<<8 | int(durBuf[3])
	if duration <= 0 || duration > 300 {
		duration = 10
	}

	deadline := time.Now().Add(time.Duration(duration) * time.Second)
	buf := s.recvPool.Get().([]byte)
	defer s.recvPool.Put(buf)
	var bytesReceived int64

	// Set read deadline
	stream.SetReadDeadline(deadline)

	for time.Now().Before(deadline) {
		n, err := stream.Read(buf)
		if err != nil {
			break
		}
		bytesReceived += int64(n)
	}

	logging.Debug("QUIC upload complete",
		logging.Field{Key: "bytes", Value: bytesReceived},
		logging.Field{Key: "mbps", Value: float64(bytesReceived*8) / float64(duration) / 1_000_000})
}

func (s *Server) handlePing(stream *quic.Stream) {
	buf := make([]byte, 64)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			return
		}
		// Echo back
		if _, err := stream.Write(buf[:n]); err != nil {
			return
		}
	}
}

// Close stops the QUIC server
func (s *Server) Close() error {
	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	logging.Info("QUIC server stopped")
	return nil
}
