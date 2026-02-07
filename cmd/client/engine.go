package client

import (
	"context"
	"errors"
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
	measureStart time.Time // set after warm-up; throughput computed from here
	totalBytes   int64
	graceBytes   int64 // bytes received/sent during warm-up (discarded)
	graceDone    int32 // CAS flag: 0 = in warm-up, 1 = measuring
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

	// Warm-up: data flows during the first WarmUp seconds but is not recorded.
	// After warm-up, counters are reset and measurement begins.
	// measureStart is set immediately; it is updated when warm-up ends.
	e.measureStart = e.startTime

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

	testCtx, cancel := context.WithTimeout(ctx, e.config.WarmUp+e.config.Duration)
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
			if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
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

func (e *TestEngine) runDownload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("D")); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
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
				if errors.Is(err, io.EOF) {
					return nil
				}
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return err
			}
			if n > 0 {
				atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
				e.addBytes(int64(n))

				if e.pastWarmUp() {
					e.recordLatency(readDuration)
					if time.Since(lastRTTSample) > rttSampleInterval {
						e.rttCollector.AddSample(readDuration.Seconds() * 1000)
						lastRTTSample = time.Now()
					}
				}
			}
		}
	}
}

func (e *TestEngine) runUpload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("U")); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
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
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return err
			}
			if n > 0 {
				atomic.AddInt64(&e.metrics.BytesSent, int64(n))
				e.addBytes(int64(n))

				if e.pastWarmUp() {
					e.recordLatency(writeDuration)
					if time.Since(lastRTTSample) > rttSampleInterval {
						e.rttCollector.AddSample(writeDuration.Seconds() * 1000)
						lastRTTSample = time.Now()
					}
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
		buf, ok := e.bufferPool.Get().([]byte)
		if !ok {
			buf = make([]byte, 64*1024)
		}
		defer e.bufferPool.Put(buf)
		lastRTTSample := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
				readStart := time.Now()
				n, err := conn.Read(buf)
				readDuration := time.Since(readStart)
				if err != nil {
					var netErr net.Error
					if errors.As(err, &netErr) && netErr.Timeout() {
						continue
					}
					return
				}
				if n > 0 {
					atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
					e.addBytes(int64(n))
					if e.pastWarmUp() {
						e.recordLatency(readDuration)
						if time.Since(lastRTTSample) > 500*time.Millisecond {
							e.rttCollector.AddSample(readDuration.Seconds() * 1000)
							lastRTTSample = time.Now()
						}
					}
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf, ok := e.bufferPool.Get().([]byte)
		if !ok {
			buf = make([]byte, 64*1024)
		}
		defer e.bufferPool.Put(buf)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
				n, err := conn.Write(buf)
				if err != nil {
					var netErr net.Error
					if errors.As(err, &netErr) && netErr.Timeout() {
						continue
					}
					return
				}
				if n > 0 {
					atomic.AddInt64(&e.metrics.BytesSent, int64(n))
					e.addBytes(int64(n))
				}
			}
		}
	}()

	wg.Wait()
	return nil
}

// addBytes gates byte recording on warm-up. During the first WarmUp seconds,
// bytes go to graceBytes (discarded). After warm-up, a one-time reset sets
// measureStart and all subsequent bytes go to totalBytes.
func (e *TestEngine) addBytes(n int64) {
	elapsed := time.Since(e.startTime)
	if elapsed < e.config.WarmUp {
		atomic.AddInt64(&e.graceBytes, n)
		return
	}
	if atomic.CompareAndSwapInt32(&e.graceDone, 0, 1) {
		// Transition: warm-up just ended. Reset counters and mark measurement start.
		atomic.StoreInt64(&e.totalBytes, 0)
		atomic.StoreInt64(&e.metrics.BytesSent, 0)
		atomic.StoreInt64(&e.metrics.BytesReceived, 0)
		e.metrics.mu.Lock()
		e.metrics.LatencySamples = e.metrics.LatencySamples[:0]
		e.metrics.mu.Unlock()
		e.measureStart = time.Now()
	}
	atomic.AddInt64(&e.totalBytes, n)
}

// pastWarmUp returns true once the warm-up period has elapsed.
func (e *TestEngine) pastWarmUp() bool {
	return atomic.LoadInt32(&e.graceDone) == 1 || time.Since(e.startTime) >= e.config.WarmUp
}

func (e *TestEngine) recordLatency(d time.Duration) {
	e.metrics.mu.Lock()
	if len(e.metrics.LatencySamples) < 10000 {
		e.metrics.LatencySamples = append(e.metrics.LatencySamples, d)
	}
	e.metrics.mu.Unlock()
}

func (e *TestEngine) GetMetrics() EngineMetrics {
	// Throughput is computed from measureStart (after warm-up), not startTime.
	measureElapsed := time.Since(e.measureStart)
	totalBytes := atomic.LoadInt64(&e.totalBytes)
	bytesSent := atomic.LoadInt64(&e.metrics.BytesSent)
	bytesRecv := atomic.LoadInt64(&e.metrics.BytesReceived)

	throughputMbps := float64(0)
	if measureElapsed.Seconds() > 0 {
		throughputMbps = float64(totalBytes*8) / measureElapsed.Seconds() / 1_000_000
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
		Elapsed:          measureElapsed,
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
