package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func (e *TestEngine) runDownload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte(cmdDownload)); err != nil {
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
	if _, err := conn.Write([]byte(cmdUpload)); err != nil {
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
	if _, err := conn.Write([]byte(cmdBidirectional)); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		e.runBidirectionalReadLoop(ctx, conn)
	}()

	go func() {
		defer wg.Done()
		e.runBidirectionalWriteLoop(ctx, conn)
	}()

	wg.Wait()
	return nil
}

func (e *TestEngine) runBidirectionalReadLoop(ctx context.Context, conn net.Conn) {
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
				if isTimeoutError(err) {
					continue
				}
				return
			}
			if n <= 0 {
				continue
			}
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

func (e *TestEngine) runBidirectionalWriteLoop(ctx context.Context, conn net.Conn) {
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
				if isTimeoutError(err) {
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
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func calculateClientLatency(samples []time.Duration) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	slices.Sort(sorted)

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
