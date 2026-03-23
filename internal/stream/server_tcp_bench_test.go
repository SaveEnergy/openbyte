package stream

import (
	"net"
	"sync"
	"testing"
)

// BenchmarkWriteRandomChunk exercises the TCP download chunk path used by writeDownloadLoop
// (slice + Write to a live TCP connection with a draining peer).
func BenchmarkWriteRandomChunk(b *testing.B) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	acceptReady := make(chan struct{})
	go func() {
		c, err := ln.AcceptTCP()
		if err != nil {
			return
		}
		close(acceptReady)
		buf := make([]byte, sendBufferSize)
		for {
			if _, err := c.Read(buf); err != nil {
				_ = c.Close()
				return
			}
		}
	}()

	client, err := net.DialTCP("tcp", nil, ln.Addr().(*net.TCPAddr))
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()
	<-acceptReady

	s := &Server{
		randomData: make([]byte, randomDataSize),
	}
	offset := 0
	chunkSize := min(sendBufferSize, len(s.randomData))

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, next, err := s.writeRandomChunk(client, offset, chunkSize)
		if err != nil {
			b.Fatal(err)
		}
		offset = next
	}
}

// BenchmarkStreamRecvBufferPool exercises TCP recv buffer pooling used by upload/echo/read paths.
func BenchmarkStreamRecvBufferPool(b *testing.B) {
	s := &Server{
		stopCh: make(chan struct{}),
		recvPool: sync.Pool{
			New: func() any {
				return make([]byte, recvBufferSize)
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		buf := s.getRecvBuffer()
		s.recvPool.Put(buf)
	}
}
