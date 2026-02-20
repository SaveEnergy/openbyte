package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

func TestCancelStream(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+cancelSuffix, nil)
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
}

func TestCancelStreamWithBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+cancelSuffix, bytes.NewReader([]byte(`{"reason":"user"}`)))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
}

func TestCancelStreamNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+"missing"+cancelSuffix, nil)
	rec := httptest.NewRecorder()
	handler.CancelStream(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}

func TestGetStreamResultsNotCompleted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+resultsSuffix, nil)
	rec := httptest.NewRecorder()
	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (not completed yet)", rec.Code, http.StatusAccepted)
	}
}

func TestGetStreamResultsDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	tb := &trackingBody{data: []byte(`{"unexpected":"payload"}`)}
	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+resultsSuffix, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusAccepted)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetStreamResultsCompleted(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	// Complete it first
	payload := map[string]any{
		"status":  "completed",
		"metrics": map[string]any{"throughput_mbps": 250},
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+completeSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.CompleteStream(rec, req, streamID)

	// Now get results
	req = httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+resultsSuffix, nil)
	rec = httptest.NewRecorder()
	handler.GetStreamResults(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusOK)
	}
}
