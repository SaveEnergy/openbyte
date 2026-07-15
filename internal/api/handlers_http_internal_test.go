package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsJSONContentType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ct   string
		want bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"application/json;charset=utf-8", true},
		{"application/JSON", true},
		{"APPLICATION/JSON", true},
		{"application/jsonp", false},
		{"application/problem+json", false},
		{"application/json; charset", false},
		{"text/plain", false},
		{"", false},
	}
	for _, tt := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if tt.ct != "" {
			req.Header.Set("Content-Type", tt.ct)
		}
		if got := isJSONContentType(req); got != tt.want {
			t.Errorf("Content-Type %q: got %v, want %v", tt.ct, got, tt.want)
		}
	}
}
