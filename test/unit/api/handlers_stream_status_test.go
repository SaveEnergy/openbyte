package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
	"github.com/saveenergy/openbyte/internal/stream"
)

const (
	statusDurationSec           = 12
	nonexistentStream           = "nonexistent"
	nonexistentStatus           = "/api/v1/stream/nonexistent/status"
	streamProtocolTCP           = "tcp"
	streamDirDownload           = "download"
	streamStatusCodeFmt         = "status code = %d, want %d"
	streamStartStatusFmt        = "start stream status = %d, want %d"
	streamDecodeStartRespFmt    = "decode start response: %v"
	streamDecodeStatusRespFmt   = "decode status response: %v"
	streamConfigTypeFmt         = "config missing or wrong type: %T"
	streamConfigDurationTypeFmt = "config.duration missing or wrong type: %T"
	streamConfigDurationWantFmt = "config.duration = %v, want %d seconds"
	streamUnexpectedPayloadJSON = `{"unexpected":"payload"}`
)

func TestGetStreamStatus(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	req.SetPathValue("id", streamID)
	rec := httptest.NewRecorder()
	handler.GetStreamStatus(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(streamStatusCodeFmt, rec.Code, http.StatusOK)
	}
}

func TestGetStreamStatusJSONContract(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	payload := map[string]any{
		handlersProtocolKey:  streamProtocolTCP,
		handlersDirectionKey: streamDirDownload,
		handlersDurationKey:  statusDurationSec,
		handlersStreamsKey:   1,
	}
	body := mustMarshalJSON(t, payload)
	req := httptest.NewRequest(http.MethodPost, streamStartPath, bytes.NewReader(body))
	req.Header.Set(contentTypeHeader, applicationJSON)
	rec := httptest.NewRecorder()
	handler.StartStream(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf(streamStartStatusFmt, rec.Code, http.StatusCreated)
	}
	var startResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&startResp); err != nil {
		t.Fatalf(streamDecodeStartRespFmt, err)
	}
	streamID, _ := startResp[handlersStreamIDKey].(string)

	statusReq := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	statusRec := httptest.NewRecorder()
	handler.GetStreamStatus(statusRec, statusReq, streamID)
	if statusRec.Code != http.StatusOK {
		t.Fatalf(streamStatusCodeFmt, statusRec.Code, http.StatusOK)
	}

	var statusResp map[string]any
	if err := json.NewDecoder(statusRec.Body).Decode(&statusResp); err != nil {
		t.Fatalf(streamDecodeStatusRespFmt, err)
	}
	cfg, ok := statusResp["config"].(map[string]any)
	if !ok {
		t.Fatalf(streamConfigTypeFmt, statusResp["config"])
	}
	duration, ok := cfg["duration"].(float64)
	if !ok {
		t.Fatalf(streamConfigDurationTypeFmt, cfg["duration"])
	}
	if int(duration) != statusDurationSec {
		t.Fatalf(streamConfigDurationWantFmt, duration, statusDurationSec)
	}
}

func TestGetStreamStatusDrainsUnexpectedBody(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)
	streamID := createTestStream(t, handler)

	tb := &trackingBody{data: []byte(streamUnexpectedPayloadJSON)}
	req := httptest.NewRequest(http.MethodGet, streamPathPrefix+streamID+statusSuffix, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.GetStreamStatus(rec, req, streamID)

	if rec.Code != http.StatusOK {
		t.Fatalf(streamStatusCodeFmt, rec.Code, http.StatusOK)
	}
	assertTrackingBodyDrained(t, tb)
}

func TestGetStreamStatusNotFound(t *testing.T) {
	mgr := stream.NewManager(10, 10)
	mgr.Start()
	defer mgr.Stop()

	handler := api.NewHandler(mgr)

	req := httptest.NewRequest(http.MethodGet, nonexistentStatus, nil)
	rec := httptest.NewRecorder()
	handler.GetStreamStatus(rec, req, nonexistentStream)

	if rec.Code != http.StatusNotFound {
		t.Fatalf(statusCodeWantFmt, rec.Code, http.StatusNotFound)
	}
}
