package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

const clientBufferSize = 64 * 1024

type TestEngine struct {
	config       *TestEngineConfig
	connections  []net.Conn
	metrics      *LocalMetrics
	networkInfo  *types.NetworkInfo
	rttCollector *types.RTTCollector
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
			New: func() any {
				return newClientBuffer()
			},
		},
	}
}

func newClientBuffer() *[]byte {
	buf := make([]byte, clientBufferSize)
	return &buf
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

	e.captureConnectionNetworkInfo()

	var wg sync.WaitGroup
	errCh := make(chan error, e.config.Streams)

	testCtx, cancel := context.WithTimeout(ctx, e.config.WarmUp+e.config.Duration)
	defer cancel()

	for i := 0; i < len(e.connections); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := e.runStreamWorker(testCtx, e.connections[idx])
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

func (e *TestEngine) runStreamWorker(ctx context.Context, conn net.Conn) error {
	switch e.config.Direction {
	case directionDownload:
		return e.runDownload(ctx, conn)
	case directionUpload:
		return e.runUpload(ctx, conn)
	case directionBidirectional:
		return e.runBidirectional(ctx, conn)
	default:
		return nil
	}
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
