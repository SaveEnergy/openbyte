package api

import (
	"bytes"
	"encoding/json"
	stdErrors "errors"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

var jsonBufPool = sync.Pool{
	New: func() any { return &bytes.Buffer{} },
}

func isJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json")
}

func validateMetricsPayload(m types.Metrics) error {
	values := []float64{
		m.ThroughputMbps, m.ThroughputAvgMbps, m.JitterMs, m.PacketLossPercent,
		m.Latency.MinMs, m.Latency.MaxMs, m.Latency.AvgMs, m.Latency.P50Ms, m.Latency.P95Ms, m.Latency.P99Ms,
	}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.ErrInvalidConfig("metrics contain non-finite values", nil)
		}
	}
	if m.ThroughputMbps < 0 || m.ThroughputAvgMbps < 0 {
		return errors.ErrInvalidConfig("metrics throughput must be >= 0", nil)
	}
	if m.BytesTransferred < 0 {
		return errors.ErrInvalidConfig("metrics bytes_transferred must be >= 0", nil)
	}
	if m.JitterMs < 0 {
		return errors.ErrInvalidConfig("metrics jitter_ms must be >= 0", nil)
	}
	if m.PacketLossPercent < 0 || m.PacketLossPercent > 100 {
		return errors.ErrInvalidConfig("metrics packet_loss_percent must be between 0 and 100", nil)
	}
	if m.Latency.MinMs < 0 || m.Latency.MaxMs < 0 || m.Latency.AvgMs < 0 || m.Latency.P50Ms < 0 || m.Latency.P95Ms < 0 || m.Latency.P99Ms < 0 {
		return errors.ErrInvalidConfig("metrics latency values must be >= 0", nil)
	}
	if m.Latency.Count < 0 {
		return errors.ErrInvalidConfig("metrics latency count must be >= 0", nil)
	}
	return nil
}

func drainRequestBody(r *http.Request) {
	if r == nil || r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
}

func respondJSONBodyError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if stdErrors.As(err, &maxErr) {
		respondJSON(w, map[string]string{"error": "request body too large"}, http.StatusRequestEntityTooLarge)
		return
	}
	respondJSON(w, map[string]string{"error": "invalid request body"}, http.StatusBadRequest)
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

func respondError(w http.ResponseWriter, err error, statusCode int) {
	var msg string
	var streamErr *errors.StreamError
	if stdErrors.As(err, &streamErr) {
		msg = streamErr.Message
	} else {
		msg = err.Error()
	}
	respondJSON(w, map[string]string{
		"error": msg,
	}, statusCode)
}
