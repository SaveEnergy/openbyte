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

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if pong, ok := resp["pong"].(bool); !ok || !pong {
		t.Fatalf("pong = %v, want true", resp["pong"])
	}
	if _, ok := resp["timestamp"].(float64); !ok {
		t.Fatalf("timestamp missing or wrong type: %T", resp["timestamp"])
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv4Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv4Want)
	}
	if ipv6, ok := resp[pingIPv6Key].(bool); !ok || ipv6 {
		t.Fatalf(pingIPv6FalseMsg, resp[pingIPv6Key])
	}
}

func TestSpeedTestHandlerPingNilResolverFallback(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.RemoteAddr = "[2001:db8::1]:4242"
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf(speedtestDecodeRespFmt, err)
	}
	if ip, ok := resp[pingClientIPKey].(string); !ok || ip != pingClientIPv6Want {
		t.Fatalf("client_ip = %v, want %s", resp[pingClientIPKey], pingClientIPv6Want)
	}
	if ipv6, ok := resp[pingIPv6Key].(bool); !ok || !ipv6 {
		t.Fatalf(pingIPv6TrueMsg, resp[pingIPv6Key])
	}
}

func TestSpeedTestHandlerPingDrainsUnexpectedBody(t *testing.T) {
	handler := api.NewSpeedTestHandler(10, 300)
	tb := &trackingUploadBody{data: bytes.Repeat([]byte("x"), 1024)}
	req := httptest.NewRequest(http.MethodGet, pingEndpoint, nil)
	req.Body = tb
	rec := httptest.NewRecorder()

	handler.Ping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(speedtestStatusFmt, rec.Code, http.StatusOK)
	}
	if tb.reads == 0 {
		t.Fatal(speedtestExpectBodyDrained)
	}
	if !tb.closed {
		t.Fatal(speedtestExpectBodyClosed)
	}
}
