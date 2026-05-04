package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/saveenergy/openbyte/internal/logging"
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

func drainRequestBody(r *http.Request) {
	if r == nil || r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
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
		logging.Warn("JSON response marshal failed",
			logging.Field{Key: "error", Value: err})
		statusCode = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"error":"internal error"}`)
	} else if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] == '\n' {
		buf.Truncate(buf.Len() - 1) // match json.Marshal output (no trailing newline)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(buf.Bytes()); err != nil {
		logging.Warn("JSON response write failed",
			logging.Field{Key: "error", Value: err})
	}
}
