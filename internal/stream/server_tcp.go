package stream

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

const (
	tcpReadDeadline        = 5 * time.Second
	tcpEchoReadDeadline    = 1 * time.Second
	deadlineRefreshOps     = 128
	echoMinBytesPerRefresh = 4096 // refresh deadline only after meaningful throughput; prevents slowloris via tiny periodic reads
)

func (s *Server) acceptTCP() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		default:
			s.tcpListener.SetDeadline(time.Now().Add(100 * time.Millisecond))
			conn, err := s.tcpListener.AcceptTCP()
			if err != nil {
				if isTimeoutError(err) {
					continue
				}
				if s.isStopping() {
					return
				}
				logging.Warn("TCP accept error", logging.Field{Key: "error", Value: err})
				continue
			}

			conn.SetNoDelay(true)
			conn.SetReadBuffer(recvBufferSize)
			conn.SetWriteBuffer(sendBufferSize)

			if atomic.AddInt64(&s.activeTCPConns, 1) > s.maxTCPConns {
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
	connCtx, connCancel := context.WithTimeout(context.Background(), s.maxConnDur)
	defer connCancel()
	go func() {
		select {
		case <-connCtx.Done():
		case <-s.stopCh:
		}
		conn.SetDeadline(time.Now())
	}()

	conn.SetReadDeadline(time.Now().Add(tcpReadDeadline))
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
	s.writeDownloadLoop(conn)
}

func (s *Server) writeDownloadLoop(conn *net.TCPConn) {
	dataLen := len(s.randomData)
	offset := 0
	chunkSize := min(sendBufferSize, dataLen)
	writesSinceDeadline := 0
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	for {
		select {
		case <-s.stopCh:
			return
		default:
			if writesSinceDeadline >= 128 {
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				writesSinceDeadline = 0
			}
			writes, nextOffset, err := s.writeRandomChunk(conn, offset, chunkSize)
			if err != nil {
				return
			}
			writesSinceDeadline += writes
			offset = nextOffset
		}
	}
}

func (s *Server) writeRandomChunk(conn *net.TCPConn, offset, chunkSize int) (int, int, error) {
	dataLen := len(s.randomData)
	if offset+chunkSize <= dataLen {
		if _, err := conn.Write(s.randomData[offset : offset+chunkSize]); err != nil {
			return 0, offset, err
		}
		nextOffset := offset + chunkSize
		if nextOffset >= dataLen {
			nextOffset = 0
		}
		return 1, nextOffset, nil
	}

	first := s.randomData[offset:]
	if _, err := conn.Write(first); err != nil {
		return 0, offset, err
	}
	writes := 1
	remaining := chunkSize - len(first)
	if remaining > 0 {
		if _, err := conn.Write(s.randomData[:remaining]); err != nil {
			return writes, offset, err
		}
		writes++
	}
	nextOffset := remaining
	if nextOffset >= dataLen {
		nextOffset = 0
	}
	return writes, nextOffset, nil
}

func (s *Server) handleUpload(conn *net.TCPConn) {
	s.readDiscardLoop(conn, tcpReadDeadline)
}

func (s *Server) readDiscardLoop(conn *net.TCPConn, deadline time.Duration) {
	buf := s.getRecvBuffer()
	defer s.recvPool.Put(buf)
	readsSinceDeadline := 0
	conn.SetReadDeadline(time.Now().Add(deadline))

	for {
		select {
		case <-s.stopCh:
			return
		default:
			if readsSinceDeadline >= deadlineRefreshOps {
				conn.SetReadDeadline(time.Now().Add(deadline))
				readsSinceDeadline = 0
			}
			_, err := conn.Read(buf)
			if err != nil {
				if isRetryableConnReadError(err) {
					continue
				}
				return
			}
			readsSinceDeadline++
		}
	}
}

func (s *Server) handleBidirectional(conn *net.TCPConn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.writeDownloadLoop(conn)
	}()

	go func() {
		defer wg.Done()
		s.readDiscardLoop(conn, 5*time.Second)
	}()

	wg.Wait()
}

func (s *Server) handleEcho(conn *net.TCPConn) {
	buf := s.getRecvBuffer()
	defer s.recvPool.Put(buf)
	bytesSinceRefresh := 0
	deadline := time.Now().Add(tcpEchoReadDeadline)
	for {
		select {
		case <-s.stopCh:
			return
		default:
			if bytesSinceRefresh >= echoMinBytesPerRefresh {
				bytesSinceRefresh = 0
				deadline = time.Now().Add(tcpEchoReadDeadline)
			}
			conn.SetReadDeadline(deadline)
			n, err := conn.Read(buf)
			if err != nil {
				if handleEchoReadError(err) {
					continue
				}
				return
			}
			bytesSinceRefresh += n
			if n > 0 {
				if _, err := conn.Write(buf[:n]); err != nil {
					return
				}
			}
		}
	}
}

func handleEchoReadError(err error) bool {
	if !errors.Is(err, io.EOF) && isTimeoutError(err) {
		return false // idle timeout; close to prevent slowloris
	}
	return isRetryableConnReadError(err)
}

func isRetryableConnReadError(err error) bool {
	if errors.Is(err, io.EOF) {
		return false
	}
	return isTimeoutError(err)
}
