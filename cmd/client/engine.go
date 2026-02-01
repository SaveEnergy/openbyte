package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type TestEngine struct {
	config       *TestEngineConfig
	connections  []net.Conn
	metrics      *LocalMetrics
	networkInfo  *types.NetworkInfo
	rttCollector *types.RTTCollector
	mu           sync.RWMutex
	running      int32
	startTime    time.Time
	totalBytes   int64
	bufferPool   sync.Pool
}

type TestEngineConfig struct {
	ServerAddr string
	Protocol   string
	Direction  string
	Duration   time.Duration
	Streams    int
	PacketSize int
	WarmUp     time.Duration
}

type LocalMetrics struct {
	BytesSent       int64
	BytesReceived   int64
	PacketsSent     int64
	PacketsReceived int64
	LatencySamples  []time.Duration
	StartTime       time.Time
	LastUpdate      time.Time
	mu              sync.RWMutex
}

func NewTestEngine(cfg *TestEngineConfig) *TestEngine {
	networkInfo := types.NewNetworkInfo()
	iface := types.GetDefaultInterface()
	if iface != "" {
		networkInfo.MTU = types.DetectMTU(iface)
	}
	networkInfo.SetClientIP(types.GetLocalIP())

	return &TestEngine{
		config:       cfg,
		connections:  make([]net.Conn, 0, cfg.Streams),
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

func (e *TestEngine) Run(ctx context.Context) error {
	atomic.StoreInt32(&e.running, 1)
	defer atomic.StoreInt32(&e.running, 0)

	e.startTime = time.Now()
	e.metrics.StartTime = e.startTime

	baseline, err := types.MeasureBaselineRTT(e.config.ServerAddr, 10, 5*time.Second)
	if err == nil {
		e.rttCollector.SetBaseline(baseline)
	}

	if err := e.createConnections(); err != nil {
		return fmt.Errorf("create connections: %w", err)
	}
	defer e.closeConnections()

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

	if len(e.connections) > 0 {
		if tcpConn, ok := e.connections[0].(*net.TCPConn); ok {
			if addr := tcpConn.RemoteAddr(); addr != nil {
				e.networkInfo.SetServerIP(addr.String())
			}
			if addr := tcpConn.LocalAddr(); addr != nil {
				localIP := addr.String()
				e.networkInfo.DetectNAT(localIP, e.networkInfo.ClientIP)
			}
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	testCtx, cancel := context.WithTimeout(ctx, e.config.Duration)
	defer cancel()

	for i := 0; i < len(e.connections); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var err error
			switch e.config.Direction {
			case "download":
				err = e.runDownload(testCtx, e.connections[idx])
			case "upload":
				err = e.runUpload(testCtx, e.connections[idx])
			case "bidirectional":
				err = e.runBidirectional(testCtx, e.connections[idx])
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

func (e *TestEngine) createConnections() error {
	for i := 0; i < e.config.Streams; i++ {
		var conn net.Conn
		var err error

		if e.config.Protocol == "udp" {
			udpAddr, err := net.ResolveUDPAddr("udp", e.config.ServerAddr)
			if err != nil {
				e.closeConnections()
				return fmt.Errorf("resolve UDP: %w", err)
			}
			conn, err = net.DialUDP("udp", nil, udpAddr)
			if err != nil {
				e.closeConnections()
				return fmt.Errorf("dial UDP: %w", err)
			}
		} else {
			conn, err = net.DialTimeout("tcp", e.config.ServerAddr, 10*time.Second)
			if err != nil {
				e.closeConnections()
				return fmt.Errorf("dial TCP: %w", err)
			}
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetNoDelay(true)
				tcpConn.SetReadBuffer(256 * 1024)
				tcpConn.SetWriteBuffer(256 * 1024)
			}
		}

		e.connections = append(e.connections, conn)
	}
	return nil
}

func (e *TestEngine) closeConnections() {
	for _, conn := range e.connections {
		if conn != nil {
			conn.Close()
		}
	}
	e.connections = nil
}

func (e *TestEngine) runWarmUp(ctx context.Context) {
	buf := make([]byte, 64*1024)
	for _, conn := range e.connections {
		go func(c net.Conn) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					c.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
					c.Read(buf)
				}
			}
		}(conn)
	}
	<-ctx.Done()
}

func (e *TestEngine) runDownload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("D")); err != nil {
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
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			readStart := time.Now()
			n, err := conn.Read(buf)
			readDuration := time.Since(readStart)
			if err != nil {
				if err == io.EOF {
					return nil
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return err
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

func (e *TestEngine) runUpload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("U")); err != nil {
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
			conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			writeStart := time.Now()
			n, err := conn.Write(buf)
			writeDuration := time.Since(writeStart)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return err
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

func (e *TestEngine) runBidirectional(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("B")); err != nil {
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
				conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := conn.Read(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
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
				conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := conn.Write(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					return
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

func (e *TestEngine) recordLatency(d time.Duration) {
	e.metrics.mu.Lock()
	if len(e.metrics.LatencySamples) < 10000 {
		e.metrics.LatencySamples = append(e.metrics.LatencySamples, d)
	}
	e.metrics.mu.Unlock()
}

func (e *TestEngine) GetMetrics() EngineMetrics {
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

func (e *TestEngine) IsRunning() bool {
	return atomic.LoadInt32(&e.running) == 1
}

type EngineMetrics struct {
	ThroughputMbps   float64
	BytesTransferred int64
	BytesSent        int64
	BytesReceived    int64
	Latency          LatencyStats
	RTT              types.RTTMetrics
	Network          *types.NetworkInfo
	JitterMs         float64
	Elapsed          time.Duration
	Running          bool
}

type LatencyStats struct {
	MinMs float64
	MaxMs float64
	AvgMs float64
	P50Ms float64
	P95Ms float64
	P99Ms float64
	Count int
}

func calculateClientLatency(samples []time.Duration) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var sum time.Duration
	for _, s := range sorted {
		sum += s
	}

	n := len(sorted)
	return LatencyStats{
		MinMs: float64(sorted[0]) / float64(time.Millisecond),
		MaxMs: float64(sorted[n-1]) / float64(time.Millisecond),
		AvgMs: float64(sum) / float64(n) / float64(time.Millisecond),
		P50Ms: float64(sorted[n*50/100]) / float64(time.Millisecond),
		P95Ms: float64(sorted[n*95/100]) / float64(time.Millisecond),
		P99Ms: float64(sorted[n*99/100]) / float64(time.Millisecond),
		Count: n,
	}
}

func calculateClientJitter(samples []time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	var sumDiff float64
	for i := 1; i < len(samples); i++ {
		diff := samples[i] - samples[i-1]
		if diff < 0 {
			diff = -diff
		}
		sumDiff += float64(diff)
	}

	return sumDiff / float64(len(samples)-1) / float64(time.Millisecond)
}
