package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/saveenergy/openbyte/pkg/types"
)

// QUICEngine handles QUIC-based speed testing
type QUICEngine struct {
	config       *TestEngineConfig
	conn         *quic.Conn
	streams      []*quic.Stream
	metrics      *LocalMetrics
	networkInfo  *types.NetworkInfo
	rttCollector *types.RTTCollector
	running      int32
	startTime    time.Time
	totalBytes   int64
	bufferPool   sync.Pool
}

// NewQUICEngine creates a new QUIC test engine
func NewQUICEngine(cfg *TestEngineConfig) *QUICEngine {
	networkInfo := types.NewNetworkInfo()
	iface := types.GetDefaultInterface()
	if iface != "" {
		networkInfo.MTU = types.DetectMTU(iface)
	}
	networkInfo.SetClientIP(types.GetLocalIP())

	return &QUICEngine{
		config:       cfg,
		streams:      make([]*quic.Stream, 0, cfg.Streams),
		networkInfo:  networkInfo,
		rttCollector: types.NewRTTCollector(100),
		metrics: &LocalMetrics{
			LatencySamples: make([]time.Duration, 0, 10000),
			StartTime:      time.Now(),
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024)
			},
		},
	}
}

// Run executes the QUIC speed test
func (e *QUICEngine) Run(ctx context.Context) error {
	atomic.StoreInt32(&e.running, 1)
	defer atomic.StoreInt32(&e.running, 0)

	e.startTime = time.Now()
	e.metrics.StartTime = e.startTime

	if err := e.connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer e.close()

	// Warm-up
	if e.config.WarmUp > 0 {
		warmUpCtx, warmUpCancel := context.WithTimeout(ctx, e.config.WarmUp)
		e.runWarmUp(warmUpCtx)
		warmUpCancel()
		atomic.StoreInt64(&e.totalBytes, 0)
		atomic.StoreInt64(&e.metrics.BytesSent, 0)
		atomic.StoreInt64(&e.metrics.BytesReceived, 0)
		e.metrics.mu.Lock()
		e.metrics.LatencySamples = e.metrics.LatencySamples[:0]
		e.metrics.mu.Unlock()
		e.startTime = time.Now()
		e.metrics.StartTime = e.startTime
	}

	if e.conn != nil {
		e.networkInfo.SetServerIP((*e.conn).RemoteAddr().String())
	}

	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	testCtx, cancel := context.WithTimeout(ctx, e.config.Duration)
	defer cancel()

	for i := 0; i < len(e.streams); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var err error
			switch e.config.Direction {
			case "download":
				err = e.runDownload(testCtx, e.streams[idx])
			case "upload":
				err = e.runUpload(testCtx, e.streams[idx])
			case "bidirectional":
				err = e.runBidirectional(testCtx, e.streams[idx])
			}
			if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				select {
				case errCh <- err:
				default:
				}
			}
		}(i)
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (e *QUICEngine) connect(ctx context.Context) error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, // Self-signed certs for speed tests
		NextProtos:         []string{"speedtest"},
	}

	quicConf := &quic.Config{
		MaxIdleTimeout:        30 * time.Second,
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		Allow0RTT:             true,
	}

	conn, err := quic.DialAddr(ctx, e.config.ServerAddr, tlsConf, quicConf)
	if err != nil {
		return fmt.Errorf("dial QUIC: %w", err)
	}
	e.conn = conn

	// Create streams
	for i := 0; i < e.config.Streams; i++ {
		stream, err := conn.OpenStreamSync(ctx)
		if err != nil {
			e.close()
			return fmt.Errorf("open stream: %w", err)
		}
		e.streams = append(e.streams, stream)
	}

	return nil
}

func (e *QUICEngine) close() {
	for _, stream := range e.streams {
		if stream != nil {
			(*stream).Close()
		}
	}
	e.streams = nil
	if e.conn != nil {
		(*e.conn).CloseWithError(0, "done")
		e.conn = nil
	}
}

func (e *QUICEngine) runWarmUp(ctx context.Context) {
	buf := make([]byte, 64*1024)
	for _, stream := range e.streams {
		go func(s *quic.Stream) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					(*s).SetReadDeadline(time.Now().Add(100 * time.Millisecond))
					(*s).Read(buf)
				}
			}
		}(stream)
	}
	<-ctx.Done()
}

func (e *QUICEngine) runDownload(ctx context.Context, stream *quic.Stream) error {
	// Send command: D + 4-byte duration
	duration := int(e.config.Duration.Seconds())
	cmd := []byte{'D', byte(duration >> 24), byte(duration >> 16), byte(duration >> 8), byte(duration)}
	if _, err := (*stream).Write(cmd); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	buf := e.bufferPool.Get().([]byte)
	defer e.bufferPool.Put(buf)

	lastRTTSample := time.Now()
	rttSampleInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			(*stream).SetReadDeadline(time.Now().Add(1 * time.Second))
			readStart := time.Now()
			n, err := (*stream).Read(buf)
			readDuration := time.Since(readStart)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				continue
			}
			if n > 0 {
				atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
				atomic.AddInt64(&e.totalBytes, int64(n))
				e.recordLatency(readDuration)

				if time.Since(lastRTTSample) > rttSampleInterval {
					e.rttCollector.AddSample(readDuration.Seconds() * 1000)
					lastRTTSample = time.Now()
				}
			}
		}
	}
}

func (e *QUICEngine) runUpload(ctx context.Context, stream *quic.Stream) error {
	// Send command: U + 4-byte duration
	duration := int(e.config.Duration.Seconds())
	cmd := []byte{'U', byte(duration >> 24), byte(duration >> 16), byte(duration >> 8), byte(duration)}
	if _, err := (*stream).Write(cmd); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	buf := e.bufferPool.Get().([]byte)
	defer e.bufferPool.Put(buf)

	lastRTTSample := time.Now()
	rttSampleInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			(*stream).SetWriteDeadline(time.Now().Add(1 * time.Second))
			writeStart := time.Now()
			n, err := (*stream).Write(buf)
			writeDuration := time.Since(writeStart)
			if err != nil {
				continue
			}
			if n > 0 {
				atomic.AddInt64(&e.metrics.BytesSent, int64(n))
				atomic.AddInt64(&e.totalBytes, int64(n))
				e.recordLatency(writeDuration)

				if time.Since(lastRTTSample) > rttSampleInterval {
					e.rttCollector.AddSample(writeDuration.Seconds() * 1000)
					lastRTTSample = time.Now()
				}
			}
		}
	}
}

func (e *QUICEngine) runBidirectional(ctx context.Context, stream *quic.Stream) error {
	// Send command: B + 4-byte duration
	duration := int(e.config.Duration.Seconds())
	cmd := []byte{'B', byte(duration >> 24), byte(duration >> 16), byte(duration >> 8), byte(duration)}
	if _, err := (*stream).Write(cmd); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := e.bufferPool.Get().([]byte)
		defer e.bufferPool.Put(buf)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				(*stream).SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := (*stream).Read(buf)
				if err != nil {
					continue
				}
				if n > 0 {
					atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
					atomic.AddInt64(&e.totalBytes, int64(n))
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := e.bufferPool.Get().([]byte)
		defer e.bufferPool.Put(buf)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				(*stream).SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := (*stream).Write(buf)
				if err != nil {
					continue
				}
				if n > 0 {
					atomic.AddInt64(&e.metrics.BytesSent, int64(n))
					atomic.AddInt64(&e.totalBytes, int64(n))
				}
			}
		}
	}()

	wg.Wait()
	return nil
}

func (e *QUICEngine) recordLatency(d time.Duration) {
	e.metrics.mu.Lock()
	if len(e.metrics.LatencySamples) < 10000 {
		e.metrics.LatencySamples = append(e.metrics.LatencySamples, d)
	}
	e.metrics.mu.Unlock()
}

// GetMetrics returns current test metrics
func (e *QUICEngine) GetMetrics() EngineMetrics {
	elapsed := time.Since(e.startTime)
	totalBytes := atomic.LoadInt64(&e.totalBytes)
	bytesSent := atomic.LoadInt64(&e.metrics.BytesSent)
	bytesRecv := atomic.LoadInt64(&e.metrics.BytesReceived)

	throughputMbps := float64(0)
	if elapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / elapsed.Seconds() / 1_000_000
	}

	e.metrics.mu.RLock()
	samples := make([]time.Duration, len(e.metrics.LatencySamples))
	copy(samples, e.metrics.LatencySamples)
	e.metrics.mu.RUnlock()

	latency := calculateClientLatency(samples)
	jitter := calculateClientJitter(samples)
	rttMetrics := e.rttCollector.GetMetrics()

	return EngineMetrics{
		ThroughputMbps:   throughputMbps,
		BytesTransferred: totalBytes,
		BytesSent:        bytesSent,
		BytesReceived:    bytesRecv,
		Latency:          latency,
		RTT:              rttMetrics,
		Network:          e.networkInfo,
		JitterMs:         jitter,
		Elapsed:          elapsed,
		Running:          atomic.LoadInt32(&e.running) == 1,
	}
}

// IsRunning returns whether the test is currently running
func (e *QUICEngine) IsRunning() bool {
	return atomic.LoadInt32(&e.running) == 1
}
