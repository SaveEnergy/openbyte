package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func newEngineForUnitTest(warmUp time.Duration) *TestEngine {
	engine := NewTestEngine(&TestEngineConfig{
		Protocol:  "tcp",
		Direction: "download",
		WarmUp:    warmUp,
	})
	engine.startTime = time.Now().Add(-2 * warmUp)
	engine.measureStart = engine.startTime
	return engine
}

func TestAddBytesWarmUpTransitionResetsCounters(t *testing.T) {
	engine := newEngineForUnitTest(50 * time.Millisecond)
	engine.startTime = time.Now()

	engine.addBytes(100)
	if got := atomic.LoadInt64(&engine.graceBytes); got != 100 {
		t.Fatalf("graceBytes = %d, want %d", got, 100)
	}
	if got := atomic.LoadInt64(&engine.totalBytes); got != 0 {
		t.Fatalf("totalBytes during warm-up = %d, want 0", got)
	}

	atomic.StoreInt64(&engine.metrics.BytesSent, 9)
	atomic.StoreInt64(&engine.metrics.BytesReceived, 11)
	engine.metrics.mu.Lock()
	engine.metrics.LatencySamples = append(engine.metrics.LatencySamples, 10*time.Millisecond)
	engine.metrics.mu.Unlock()

	engine.startTime = time.Now().Add(-100 * time.Millisecond)
	engine.addBytes(200)

	if got := atomic.LoadInt64(&engine.totalBytes); got != 200 {
		t.Fatalf("totalBytes after warm-up = %d, want %d", got, 200)
	}
	if got := atomic.LoadInt64(&engine.metrics.BytesSent); got != 0 {
		t.Fatalf("BytesSent reset = %d, want 0", got)
	}
	if got := atomic.LoadInt64(&engine.metrics.BytesReceived); got != 0 {
		t.Fatalf("BytesReceived reset = %d, want 0", got)
	}
	engine.metrics.mu.RLock()
	latencyCount := len(engine.metrics.LatencySamples)
	engine.metrics.mu.RUnlock()
	if latencyCount != 0 {
		t.Fatalf("LatencySamples len = %d, want 0", latencyCount)
	}
}

func TestRunDownloadReadsDataAndTracksBytes(t *testing.T) {
	engine := newEngineForUnitTest(0)
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]
		_, _ = serverConn.Write([]byte("warmup-transition-chunk"))
		time.Sleep(20 * time.Millisecond)
		_, _ = serverConn.Write([]byte("measured-chunk"))
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	if err := engine.runDownload(ctx, clientConn); err != nil {
		t.Fatalf("runDownload: %v", err)
	}

	if cmd := <-gotCommand; cmd != 'D' {
		t.Fatalf("download command = %q, want %q", cmd, 'D')
	}
	if got := atomic.LoadInt64(&engine.metrics.BytesReceived); got <= 0 {
		t.Fatalf("BytesReceived = %d, want > 0", got)
	}
	if got := atomic.LoadInt64(&engine.totalBytes); got <= 0 {
		t.Fatalf("totalBytes = %d, want > 0", got)
	}
}

func TestRunUploadSendsCommandAndPayloadUntilContextDone(t *testing.T) {
	engine := newEngineForUnitTest(0)
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	gotPayload := make(chan struct{}, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]

		buf := make([]byte, 1024)
		for {
			n, err := serverConn.Read(buf)
			if n > 0 {
				select {
				case gotPayload <- struct{}{}:
				default:
				}
			}
			if err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := engine.runUpload(ctx, clientConn)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("runUpload error = %v, want context cancellation", err)
	}

	if cmd := <-gotCommand; cmd != 'U' {
		t.Fatalf("upload command = %q, want %q", cmd, 'U')
	}
	select {
	case <-gotPayload:
	case <-time.After(1 * time.Second):
		t.Fatal("expected upload payload bytes")
	}
}

func TestRunBidirectionalSendsCommandAndReturnsOnPeerClose(t *testing.T) {
	engine := newEngineForUnitTest(0)
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)
	if err := engine.runBidirectional(ctx, clientConn); err != nil {
		t.Fatalf("runBidirectional: %v", err)
	}
	if cmd := <-gotCommand; cmd != 'B' {
		t.Fatalf("bidirectional command = %q, want %q", cmd, 'B')
	}
}

func TestRunStreamWorkerUnknownDirectionNoop(t *testing.T) {
	engine := newEngineForUnitTest(0)
	engine.config.Direction = "unknown"
	if err := engine.runStreamWorker(context.Background(), nil); err != nil {
		t.Fatalf("runStreamWorker unknown direction: %v", err)
	}
}

func TestRunStreamWorkerDispatchDownload(t *testing.T) {
	engine := newEngineForUnitTest(0)
	engine.config.Direction = "download"
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]
		_, _ = serverConn.Write([]byte("stream-worker-download"))
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)
	if err := engine.runStreamWorker(ctx, clientConn); err != nil {
		t.Fatalf("runStreamWorker(download): %v", err)
	}
	if cmd := <-gotCommand; cmd != 'D' {
		t.Fatalf("download command = %q, want %q", cmd, 'D')
	}
}

func TestRunStreamWorkerDispatchUpload(t *testing.T) {
	engine := newEngineForUnitTest(0)
	engine.config.Direction = "upload"
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]

		buf := make([]byte, 1024)
		_, _ = serverConn.Read(buf)
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)
	err := engine.runStreamWorker(ctx, clientConn)
	if err == nil {
		t.Fatal("runStreamWorker(upload) error = nil, want non-nil from peer close")
	}
	if cmd := <-gotCommand; cmd != 'U' {
		t.Fatalf("upload command = %q, want %q", cmd, 'U')
	}
}

func TestRunStreamWorkerDispatchBidirectional(t *testing.T) {
	engine := newEngineForUnitTest(0)
	engine.config.Direction = "bidirectional"
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	gotCommand := make(chan byte, 1)
	go func() {
		cmd := make([]byte, 1)
		_, _ = io.ReadFull(serverConn, cmd)
		gotCommand <- cmd[0]
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(cancel)
	if err := engine.runStreamWorker(ctx, clientConn); err != nil {
		t.Fatalf("runStreamWorker(bidirectional): %v", err)
	}
	if cmd := <-gotCommand; cmd != 'B' {
		t.Fatalf("bidirectional command = %q, want %q", cmd, 'B')
	}
}

func TestCreateConnectionsFailureClosesExistingConnections(t *testing.T) {
	engine := NewTestEngine(&TestEngineConfig{
		Protocol:   "udp",
		ServerAddr: "invalid-address",
		Streams:    1,
		WarmUp:     0,
	})
	existing := &stubConn{}
	engine.connections = []net.Conn{existing}

	err := engine.createConnections()
	if err == nil {
		t.Fatal("createConnections error = nil, want failure")
	}
	if !existing.closed {
		t.Fatal("expected existing connection to be closed on failure")
	}
	if len(engine.connections) != 0 {
		t.Fatalf("connections len = %d, want 0 after cleanup", len(engine.connections))
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestIsTimeoutError(t *testing.T) {
	if !isTimeoutError(timeoutErr{}) {
		t.Fatal("isTimeoutError(timeoutErr{}) = false, want true")
	}
	if isTimeoutError(io.EOF) {
		t.Fatal("isTimeoutError(io.EOF) = true, want false")
	}
}

type stubConn struct {
	closed bool
}

func (s *stubConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (s *stubConn) Write(_ []byte) (int, error)        { return 0, io.EOF }
func (s *stubConn) Close() error                       { s.closed = true; return nil }
func (s *stubConn) LocalAddr() net.Addr                { return stubAddr("local") }
func (s *stubConn) RemoteAddr() net.Addr               { return stubAddr("remote") }
func (s *stubConn) SetDeadline(_ time.Time) error      { return nil }
func (s *stubConn) SetReadDeadline(_ time.Time) error  { return nil }
func (s *stubConn) SetWriteDeadline(_ time.Time) error { return nil }

type stubAddr string

func (s stubAddr) Network() string { return "stub" }
func (s stubAddr) String() string  { return string(s) }
