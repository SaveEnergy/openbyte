package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saveenergy/openbyte/internal/api"
)

func TestSpeedTestHandlerPingResponseShape(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(speedtestContentTypeKey); got != jsonContentType {
		t.Fatalf("content-type = %q, want %q", got, jsonContentType)
	}
	if got := rec.Header().Get(speedtestCacheControlKey); got != noStoreCacheControl {
		t.Fatalf("cache-control = %q, want %q", got, noStoreCacheControl)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q, want empty without Origin request header", got)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if len(resp) != 1 {
		t.Fatalf("plain ping fields = %v, want only client_ip", resp)
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv4Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv4Want)
	}
}

func TestSpeedTestHandlerPingReturnsIPv6Address(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.RemoteAddr = "[2001:db8::1]:4242"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if len(resp) != 1 {
		t.Fatalf("plain ping fields = %v, want only client_ip", resp)
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv6Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv6Want)
	}
}

func TestSpeedTestHandlerPingDoesNotReadUnexpectedBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if tb.reads != 0 {
		t.Fatalf("request body reads = %d, want 0", tb.reads)
	}
	if tb.closed {
		t.Fatal("expected the server to own request body cleanup")
	}
}
