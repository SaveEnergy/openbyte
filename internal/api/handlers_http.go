package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
)

var jsonBufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

const (
	jsonContentTypePrefix    = "application/json"
	jsonContentTypePrefixLen = len(jsonContentTypePrefix)
)

func isJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return len(ct) >= jsonContentTypePrefixLen && ct[:jsonContentTypePrefixLen] == jsonContentTypePrefix
}

func respondJSON(w http.ResponseWriter, data any, statusCode int) {
	buf, ok := jsonBufPool.Get().(*bytes.Buffer)
	if !ok {
		buf = &bytes.Buffer{}
	}
	defer func() {
		buf.Reset()
		jsonBufPool.Put(buf)
	}()
	buf.Grow(256)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		slog.Warn("JSON response marshal failed", "error", err)
		statusCode = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"error":"internal error"}`)
	} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
		buf.Truncate(buf.Len() - 1) // match json.Marshal output (no trailing newline)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Warn("JSON response write failed", "error", err)
	}
}
