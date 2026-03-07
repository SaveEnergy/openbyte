package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync/atomic"
	"time"
)

const (
	sendCmdFmt              = "send command: %w"
	bidirectionalLoopTimout = 500 * time.Millisecond
)

func (e *TestEngine) runDownload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte(cmdDownload)); err != nil {
		return fmt.Errorf(sendCmdFmt, err)
	}
	return e.runReadLoop(ctx, conn, 1*time.Second, func(n int, d time.Duration) {
		atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
		e.addBytes(int64(n))
		if e.pastWarmUp() {
			e.recordLatency(d)
		}
	})
}

func (e *TestEngine) runReadLoop(ctx context.Context, conn net.Conn, timeout time.Duration, onRead func(n int, readDuration time.Duration)) error {
	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
	defer e.bufferPool.Put(buf)
	lastRTTSample := time.Now()
	rttSampleInterval := 500 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readDuration, err := readWithDeadline(conn, buf, timeout)
		if err != nil {
			done, retry, retErr := handleReadLoopError(err)
			if done {
				return nil
			}
			if retry {
				continue
			}
			return retErr
		}
		if n <= 0 {
			continue
		}
		onRead(n, readDuration)
		lastRTTSample = updateRTTSampleIfNeeded(
			e,
			lastRTTSample,
			rttSampleInterval,
			readDuration,
		)
	}
}

func handleReadLoopError(err error) (done bool, retry bool, retErr error) {
	if errors.Is(err, io.EOF) {
		return true, false, nil
	}
	if isTimeoutError(err) {
		return false, true, nil
	}
	return false, false, err
}

func readWithDeadline(conn net.Conn, buf []byte, timeout time.Duration) (int, time.Duration, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	readStart := time.Now()
	n, err := conn.Read(buf)
	return n, time.Since(readStart), err
}

func updateRTTSampleIfNeeded(
	e *TestEngine,
	lastRTTSample time.Time,
	interval time.Duration,
	readDuration time.Duration,
) time.Time {
	if time.Since(lastRTTSample) <= interval || !e.pastWarmUp() {
		return lastRTTSample
	}
	e.rttCollector.AddSample(readDuration.Seconds() * 1000)
	return time.Now()
}

func (e *TestEngine) runUpload(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte(cmdUpload)); err != nil {
		return fmt.Errorf(sendCmdFmt, err)
	}
	return e.runWriteLoop(ctx, conn, 1*time.Second, func(n int, d time.Duration) {
		atomic.AddInt64(&e.metrics.BytesSent, int64(n))
		e.addBytes(int64(n))
		if e.pastWarmUp() {
			e.recordLatency(d)
		}
	})
}

func (e *TestEngine) runWriteLoop(ctx context.Context, conn net.Conn, timeout time.Duration, onWrite func(n int, writeDuration time.Duration)) error {
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
			conn.SetWriteDeadline(time.Now().Add(timeout))
			writeStart := time.Now()
			n, err := conn.Write(buf)
			writeDuration := time.Since(writeStart)
			if err != nil {
				if isTimeoutError(err) {
					continue
				}
				return err
			}
			if n > 0 {
				onWrite(n, writeDuration)
				if time.Since(lastRTTSample) > rttSampleInterval && e.pastWarmUp() {
					e.rttCollector.AddSample(writeDuration.Seconds() * 1000)
					lastRTTSample = time.Now()
				}
			}
		}
	}
}

func (e *TestEngine) runBidirectional(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte(cmdBidirectional)); err != nil {
		return fmt.Errorf(sendCmdFmt, err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- e.runBidirectionalReadLoop(runCtx, conn)
	}()
	go func() {
		errCh <- e.runBidirectionalWriteLoop(runCtx, conn)
	}()

	var runErr error
	for range 2 {
		err := <-errCh
		if shouldIgnoreBidirectionalError(err) {
			continue
		}
		if runErr == nil {
			runErr = err
			cancel()
		}
	}
	if runErr != nil {
		return runErr
	}
	return ctx.Err()
}

func (e *TestEngine) runBidirectionalReadLoop(ctx context.Context, conn net.Conn) error {
	return e.runBidiReadLoop(ctx, conn, func(n int, d time.Duration) {
		atomic.AddInt64(&e.metrics.BytesReceived, int64(n))
		e.addBytes(int64(n))
		if e.pastWarmUp() {
			e.recordLatency(d)
		}
	})
}

func (e *TestEngine) runBidiReadLoop(ctx context.Context, conn net.Conn, onRead func(n int, d time.Duration)) error {
	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
	defer e.bufferPool.Put(buf)
	lastRTTSample := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readDuration, err := readWithDeadline(conn, buf, bidirectionalLoopTimout)
		if err != nil {
			retry, retErr := handleBidirectionalLoopError(ctx, err)
			if retry {
				continue
			}
			return retErr
		}
		if n <= 0 {
			continue
		}
		onRead(n, readDuration)
		lastRTTSample = updateRTTSampleIfNeeded(
			e,
			lastRTTSample,
			bidirectionalLoopTimout,
			readDuration,
		)
	}
}

func (e *TestEngine) runBidirectionalWriteLoop(ctx context.Context, conn net.Conn) error {
	buf, ok := e.bufferPool.Get().([]byte)
	if !ok {
		buf = make([]byte, 64*1024)
	}
	defer e.bufferPool.Put(buf)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn.SetWriteDeadline(time.Now().Add(bidirectionalLoopTimout))
		n, err := conn.Write(buf)
		if err != nil {
			retry, retErr := handleBidirectionalLoopError(ctx, err)
			if retry {
				continue
			}
			return retErr
		}
		if n > 0 {
			atomic.AddInt64(&e.metrics.BytesSent, int64(n))
			e.addBytes(int64(n))
		}
	}
}

func handleBidirectionalLoopError(ctx context.Context, err error) (retry bool, retErr error) {
	if isTimeoutError(err) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		return true, nil
	}
	return false, err
}

func shouldIgnoreBidirectionalError(err error) bool {
	return err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
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
