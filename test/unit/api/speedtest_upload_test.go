package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestSpeedTestUploadReportsBytes(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	payload := bytes.Repeat([]byte("a"), 256*1024)
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader(payload))
	req.Header.Set(speedtestContentTypeKey, octetStreamType)
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}

	var resp struct {
		Bytes int64 `json:"bytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if resp.Bytes != int64(len(payload)) {
		t.Fatalf("bytes = %d, want %d", resp.Bytes, len(payload))
	}
}

func TestSpeedTestUploadHandlesReadError(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)

	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = io.NopCloser(&errReader{})
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
}

func TestSpeedTestUploadReadErrorDrainsBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &failingTrackingBody{}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
	if tb.reads == 0 {
		t.Fatal("expected upload body to be read")
	}
	if !tb.closed {
		t.Fatal("expected upload body to be closed")
	}
}

func TestUploadConcurrentLimitAndRelease(t *testing.T) {
	maxConcurrent := 2
	handler := api.NewSpeedTestHandler(maxConcurrent, 300)

	// Fill all upload slots with long-running uploads using signal readers
	cancels := make([]context.CancelFunc, maxConcurrent)
	done := make(chan struct{}, maxConcurrent)
	started := make([]chan struct{}, maxConcurrent)

	for i := range maxConcurrent {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		started[i] = make(chan struct{})

		go func(ch chan struct{}) {
			defer func() { done <- struct{}{} }()
			// Slow reader that blocks until context is cancelled
			pr, pw := io.Pipe()
			go func() {
				<-ctx.Done()
				pw.Close()
			}()
			body := &signalReader{ReadCloser: io.NopCloser(pr), started: ch}
			req := httptest.NewRequest(http.MethodPost, uploadEndpoint, body)
			rec := httptest.NewRecorder()
			handler.Upload(rec, req)
		}(started[i])
	}

	// Wait for all goroutines to actually enter the handler
	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitUploadStartTimeout)
		}
	}

	// New upload should get 503
	reqOver := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader([]byte("data")))
	recOver := httptest.NewRecorder()
	handler.Upload(recOver, reqOver)
	if recOver.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, recOver.Code)
	}

	// Cancel all running uploads
	for _, cancel := range cancels {
		cancel()
	}
	for range maxConcurrent {
		<-done
	}

	// After cancellation, new upload should succeed
	reqAfter := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader(bytes.Repeat([]byte("x"), 1024)))
	recAfter := httptest.NewRecorder()
	handler.Upload(recAfter, reqAfter)
	if recAfter.Code != http.StatusOK {
		t.Fatalf(speedtestExpected200AfterCancelFmt, recAfter.Code)
	}
}

func TestUploadPerIPLimitRejectsSameIPAllowsDifferentIP(t *testing.T) {
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
			pr, pw := io.Pipe()
			go func() {
				<-ctx.Done()
				_ = pw.Close()
			}()
			body := &signalReader{ReadCloser: io.NopCloser(pr), started: ch}
			req := httptest.NewRequest(http.MethodPost, uploadEndpoint, body)
			req.RemoteAddr = sameIP
			rec := httptest.NewRecorder()
			handler.Upload(rec, req)
		}(started[i])
	}

	for _, ch := range started {
		select {
		case <-ch:
		case <-time.After(speedtestWaitTimeout):
			t.Fatal(speedtestWaitUploadStartTimeout)
		}
	}

	sameReq := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader([]byte("data")))
	sameReq.RemoteAddr = sameIP
	sameRec := httptest.NewRecorder()
	handler.Upload(sameRec, sameReq)
	if sameRec.Code != statusServiceUnavailable {
		t.Fatalf(speedtestExpected503AtLimitFmt, sameRec.Code)
	}

	otherReq := httptest.NewRequest(http.MethodPost, uploadEndpoint, bytes.NewReader(bytes.Repeat([]byte("x"), 1024)))
	otherReq.RemoteAddr = otherIP
	otherRec := httptest.NewRecorder()
	handler.Upload(otherRec, otherReq)
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

func TestUploadAtCapacityDrainsBodyBefore503(t *testing.T) {
	handler := api.NewSpeedTestHandler(0, 300)

	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 4096)}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

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

func TestUploadRespectsReadDeadlineWhenBodyStalls(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 1)
	body := &blockingUploadBody{closed: make(chan struct{})}
	req := httptest.NewRequest(http.MethodPost, uploadEndpoint, nil)
	req.Body = body
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.Upload(rec, req)
	elapsed := time.Since(start)

	if rec.Code != statusInternalServerErr {
		t.Fatalf(speedtestStatusFmt, rec.Code, statusInternalServerErr)
	}
	if elapsed > speedtestWaitTimeout {
		t.Fatalf("upload read deadline not enforced, elapsed=%v", elapsed)
	}
}
