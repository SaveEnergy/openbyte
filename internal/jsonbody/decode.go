// Package jsonbody decodes HTTP request bodies as a single JSON object.
package jsonbody

import (
	"encoding/json"
	stdErrors "errors"
	"io"
	"net/http"
)

// ErrTrailingJSON is returned when the body contains valid JSON followed by
// additional non-EOF data (more than one top-level JSON value).
var ErrTrailingJSON = stdErrors.New("request body must contain a single JSON object")

// DecodeSingleObject reads r.Body into dst using json.Decoder with
// DisallowUnknownFields. When limit > 0, the body is wrapped with
// http.MaxBytesReader using w for the error response path.
//
// On any decode error, the remainder of the body is drained. The caller must
// not read r.Body after an error return.
func DecodeSingleObject(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	if limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		return err
	}
	if err := dec.Decode(&struct{}{}); !stdErrors.Is(err, io.EOF) {
		_, _ = io.Copy(io.Discard, r.Body)
		return ErrTrailingJSON
	}
	return nil
}
