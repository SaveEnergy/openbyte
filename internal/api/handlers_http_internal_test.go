package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondJSONNoTrailingNewline(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	respondJSON(w, data, http.StatusOK)

	body := w.Body.Bytes()
	if len(body) > 0 && body[len(body)-1] == '\n' {
		t.Errorf("respondJSON added trailing newline; want exact json.Marshal output")
	}
}
