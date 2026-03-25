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
		{"text/plain", false},
		{"", false},
		{"application/JSON", false},
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

func TestRespondJSONNoTrailingNewline(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	respondJSON(w, data, http.StatusOK)

	body := w.Body.Bytes()
	if len(body) > 0 && body[len(body)-1] == '\n' {
		t.Errorf("respondJSON added trailing newline; want exact json.Marshal output")
	}
}
