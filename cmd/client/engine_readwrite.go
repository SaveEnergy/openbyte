package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"
)

const sendCmdFmt = "send command: %w"

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
