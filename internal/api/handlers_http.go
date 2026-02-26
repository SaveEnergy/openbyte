package api

import (
	"encoding/json"
	stdErrors "errors"
	"io"
	"math"
	"net"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	if limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		io.Copy(io.Discard, r.Body)
		return err
	}
	if err := decoder.Decode(&struct{}{}); !stdErrors.Is(err, io.EOF) {
		io.Copy(io.Discard, r.Body)
		return stdErrors.New("request body must contain a single JSON object")
	}
	return nil
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
	payload, err := json.Marshal(data)
	if err != nil {
		logging.Warn("JSON response marshal failed",
			logging.Field{Key: "error", Value: err})
		statusCode = http.StatusInternalServerError
		payload = []byte(`{"error":"internal error"}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(payload); err != nil {
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

func normalizeHost(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	trimmed := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		trimmed = h
		if strings.Contains(h, ":") && strings.Contains(host, "[") {
			trimmed = "[" + h + "]"
		}
	}
	if trimmed == "" || trimmed == "localhost" {
		return "127.0.0.1"
	}
	return trimmed
}

func requestScheme(r *http.Request, cfg *config.Config) string {
	if r == nil {
		return "http"
	}
	if cfg != nil && cfg.TrustProxyHeaders {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			if strings.EqualFold(proto, "https") {
				return "https"
			}
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func responseHost(r *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if cfg.PublicHost != "" {
			return cfg.PublicHost
		}
		if !cfg.TrustProxyHeaders {
			return normalizeHost(cfg.BindAddress)
		}
	}
	return normalizeHost(r.Host)
}
