package api_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/types"
)

func TestReportMetrics(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	metrics := types.Metrics{ThroughputMbps: 500, BytesTransferred: 1024}
	body := mustMarshalJSON(t, metrics)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+metricsSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

func TestReportMetricsRejectsWrongContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	body := mustMarshalJSON(t, types.Metrics{ThroughputMbps: 500})
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+metricsSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, textPlain)
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestReportMetricsRequiresContentType(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	body := mustMarshalJSON(t, types.Metrics{ThroughputMbps: 500})
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+metricsSuffix, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestReportMetricsRejectsUnknownFields(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+metricsSuffix, strings.NewReader(`{"throughput_mbps":100,"unknown":1}`))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestReportMetricsValidation(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	metrics := types.Metrics{
		ThroughputMbps: -1,
	}
	body := mustMarshalJSON(t, metrics)
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+streamID+metricsSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, streamID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusBadRequest)
	}
}

func TestReportMetricsEarlyRejectsDrainBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	tests := []struct {
		name        string
		method      string
		contentType string
		wantStatus  int
	}{
		{name: "wrong method", method: http.MethodGet, contentType: applicationJSON, wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong content type", method: http.MethodPost, contentType: textPlain, wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &trackingBody{data: []byte(`{"throughput_mbps":123}`)}
			req := httptest.NewRequest(tt.method, streamPathPrefix+"any"+metricsSuffix, nil)
			req.Body = tb
			if tt.contentType != "" {
				req.Header.Set(contentTypeHeader, tt.contentType)
			}
			rec := httptest.NewRecorder()

			handler.ReportMetrics(rec, req, "any")

			if rec.Code != tt.wantStatus {
				t.Fatalf(statusCodeWantFmt, rec.Code, tt.wantStatus)
			}
			assertTrackingBodyDrained(t, tb)
		})
	}
}

func TestReportMetricsNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	body := mustMarshalJSON(t, types.Metrics{})
	req := httptest.NewRequest(http.MethodPost, streamPathPrefix+"missing"+metricsSuffix, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.ReportMetrics(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}
