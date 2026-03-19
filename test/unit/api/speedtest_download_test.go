package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestSpeedTestDownloadWritesData(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	t.Cleanup(cancel)

	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(speedtestContentTypeKey); got != octetStreamType {
		t.Fatalf("content-type = %q, want %q", got, octetStreamType)
	}
	if rec.Header().Get(speedtestCacheControlKey) == "" {
		t.Fatalf("cache-control header missing")
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected non-zero download body")
	}
}

func TestDownloadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all slots with long-running downloads using signal writers
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)
	started := make([]chan struct{}, maxConcurrent)

	for i := range maxConcurrent {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})

		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur60Chunk, nil)
			req = req.WithContext(ctx)
			sw := &signalWriter{ResponseRecorder: httptest.NewRecorder(), started: ch}
			handler.Download(sw, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitDownloadStartTimeout)
		}
	}

	// New download should get 503
	reqOver := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	recOver := httptest.NewRecorder()
	handler.Download(recOver, reqOver)
	if recOver.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, recOver.Code)
	}

	// Cancel all running downloads (simulates user pressing cancel)
	for _, cancel := range cancels {
		cancel()
	}
	for range maxConcurrent {
		<-done
	}

	// After cancellation, new download should succeed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)
	reqAfter := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	reqAfter = reqAfter.WithContext(ctx)
	recAfter := httptest.NewRecorder()
	handler.Download(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(speedtestExpected200AfterCancelFmt, recAfter.Code)
	}
}

func TestDownloadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Download(rec, req)

	if rec.Code != statusServiceUnavailable {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusServiceUnavailable)
	}
	if tb.reads == 0 {
		t.Fatalf(speedtestExpectReqBodyDrained503)
	}
	if !tb.closed {
		t.Fatalf(speedtestExpectReqBodyClosed)
	}
}

func TestDownloadPerIPLimitRejectsSameIPAllowsDifferentIP(t *testing.T) {
	handler := api.NewSpeedTestHandler(4, 300)
	handler.SetMaxConcurrentPerIP(2)

	sameIP := "203.0.113.10:1234"
	otherIP := "203.0.113.11:1234"
	cancels := make([]context.CancelFunc, 2)
	done := make(chan struct{}, 2)
	started := make([]chan struct{}, 2)

	for i := range cancels {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})
		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur60Chunk, nil)
			req = req.WithContext(ctx)
			req.RemoteAddr = sameIP
			sw := &signalWriter{ResponseRecorder: httptest.NewRecorder(), started: ch}
			handler.Download(sw, req)
		}(started[i])
	}

	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitDownloadStartTimeout)
		}
	}

	sameReq := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	sameReq.RemoteAddr = sameIP
	sameRec := httptest.NewRecorder()
	handler.Download(sameRec, sameReq)
	if sameRec.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, sameRec.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	t.Cleanup(cancel)
	otherReq := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur1Chunk, nil)
	otherReq = otherReq.WithContext(ctx)
	otherReq.RemoteAddr = otherIP
	otherRec := httptest.NewRecorder()
	handler.Download(otherRec, otherReq)
	if otherRec.Code != http.StatusOK {
		t.Fatalf(speedtestExpected200AfterCancelFmt, otherRec.Code)
	}

	for _, cancel := range cancels {
		cancel()
	}
	for range cancels {
		<-done
	}
}

func TestDownloadValidationRejectsDrainBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	tests := []string{
		downloadEndpointBase + speedtestQueryDurZero,
		downloadEndpointBase + speedtestQueryChunkBad,
	}
	for _, u := range tests {
		tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
		req := httptest.NewRequest(http.MethodGet, u, nil)
		req.Body = tb
		rec := httptest.NewRecorder()

		handler.Download(rec, req)

		if rec.Code != statusBadRequest {
			t.Fatalf(speedtestURLStatusFmt, u, rec.Code, statusBadRequest)
		}
		if tb.reads == 0 {
			t.Fatalf(speedtestURLBodyDrainedFmt, u)
		}
		if !tb.closed {
			t.Fatalf(speedtestURLBodyClosedFmt, u)
		}
	}
}

func TestDownloadRespectsMaxDuration(t *testing.T) {
	// maxDurationSec=5: duration=5 should work, duration=10 should be clamped to default
	handler := api.NewSpeedTestHandler(10, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// duration=5 (within max) should be accepted
	req := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur5Chunk, nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.Download(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duration=5 with max=5: status = %d, want 200", rec.Code)
	}

	// duration=10 (above max=5) should be rejected with 400
	req2 := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryDur10Chunk, nil)
	rec2 := httptest.NewRecorder()
	handler.Download(rec2, req2)
	if rec2.Code != statusBadRequest {
		t.Fatalf(speedtestDurationTooHighFmt, rec2.Code)
	}

	// invalid chunk should be rejected with 400
	req3 := httptest.NewRequest(http.MethodGet, downloadEndpointBase+speedtestQueryChunkABC, nil)
	rec3 := httptest.NewRecorder()
	handler.Download(rec3, req3)
	if rec3.Code != statusBadRequest {
		t.Fatalf(speedtestChunkABCFmt, rec3.Code)
	}
}
