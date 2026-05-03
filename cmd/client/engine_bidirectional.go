package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

const bidirectionalLoopTimout = 500 * time.Millisecond

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
	bufPtr, ok := e.bufferPool.Get().(*[]byte)
	if !ok || bufPtr == nil || len(*bufPtr) < clientBufferSize {
		bufPtr = newClientBuffer()
	}
	buf := *bufPtr
	defer e.bufferPool.Put(bufPtr)
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
	bufPtr, ok := e.bufferPool.Get().(*[]byte)
	if !ok || bufPtr == nil || len(*bufPtr) < clientBufferSize {
		bufPtr = newClientBuffer()
	}
	buf := *bufPtr
	defer e.bufferPool.Put(bufPtr)
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
