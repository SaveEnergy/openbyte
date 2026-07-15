package api

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
)

func isJSONContentType(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == contentTypeJSON
}

func respondJSON(w http.ResponseWriter, data any, statusCode int) {
	payload, err := json.Marshal(data)
	if err != nil {
		slog.Warn("JSON response marshal failed", "error", err)
		statusCode = http.StatusInternalServerError
		payload = []byte(`{"error":"internal error"}`)
	}
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(statusCode)
	if _, err := w.Write(payload); err != nil {
		slog.Warn("JSON response write failed", "error", err)
	}
}
