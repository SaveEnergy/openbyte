package api

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/results"
)

const maxResultBodyBytes = 4096

var errTrailingJSON = errors.New("request body must contain a single JSON object")

type resultHandler struct {
	store *results.Store
}

func newResultHandler(store *results.Store) *resultHandler {
	if store == nil {
		return nil
	}
	return &resultHandler{store: store}
}

type saveResultRequest struct {
	DownloadMbps     float64 `json:"download_mbps"`
	UploadMbps       float64 `json:"upload_mbps"`
	LatencyMs        float64 `json:"latency_ms"`
	JitterMs         float64 `json:"jitter_ms"`
	LoadedLatencyMs  float64 `json:"loaded_latency_ms"`
	BufferbloatGrade string  `json:"bufferbloat_grade"`
	IPv4             string  `json:"ipv4"`
	IPv6             string  `json:"ipv6"`
	ServerName       string  `json:"server_name"`
}

type saveResultResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func (h *resultHandler) save(w http.ResponseWriter, r *http.Request) {
	if contentType := r.Header.Get("Content-Type"); contentType != "" && !isJSONContentType(r) {
		drainRequestBody(r)
		respondResultError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req saveResultRequest
	if err := decodeSingleObject(w, r, &req, maxResultBodyBytes); err != nil {
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxBytesErr):
			respondResultError(w, "request body too large", http.StatusRequestEntityTooLarge)
		case errors.Is(err, errTrailingJSON):
			respondResultError(w, "request body must contain a single JSON object", http.StatusBadRequest)
		default:
			respondResultError(w, "invalid request body", http.StatusBadRequest)
		}
		return
	}

	if req.DownloadMbps < 0 || req.UploadMbps < 0 || req.LatencyMs < 0 ||
		req.JitterMs < 0 || req.LoadedLatencyMs < 0 {
		respondResultError(w, "numeric fields must be >= 0", http.StatusBadRequest)
		return
	}
	if hasNonFinite(req.DownloadMbps, req.UploadMbps, req.LatencyMs, req.JitterMs, req.LoadedLatencyMs) {
		respondResultError(w, "numeric fields must be finite", http.StatusBadRequest)
		return
	}
	if req.DownloadMbps > 100000 || req.UploadMbps > 100000 ||
		req.LatencyMs > 60000 || req.JitterMs > 60000 || req.LoadedLatencyMs > 60000 {
		respondResultError(w, "values out of reasonable range", http.StatusBadRequest)
		return
	}
	if len(req.ServerName) > 200 || len(req.IPv4) > 45 || len(req.IPv6) > 45 ||
		len(req.BufferbloatGrade) > 5 {
		respondResultError(w, "field too long", http.StatusBadRequest)
		return
	}

	id, err := h.store.Save(r.Context(), results.Result{
		DownloadMbps:     req.DownloadMbps,
		UploadMbps:       req.UploadMbps,
		LatencyMs:        req.LatencyMs,
		JitterMs:         req.JitterMs,
		LoadedLatencyMs:  req.LoadedLatencyMs,
		BufferbloatGrade: req.BufferbloatGrade,
		IPv4:             req.IPv4,
		IPv6:             req.IPv6,
		ServerName:       req.ServerName,
	})
	if err != nil {
		logging.Warn("results: save failed", logging.Field{Key: "error", Value: err})
		msg, code := mapSaveStoreError(err)
		respondResultError(w, msg, code)
		return
	}

	respondResultJSON(w, saveResultResponse{ID: id, URL: "/results/" + id}, http.StatusCreated)
}

func (h *resultHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validResultID(id) {
		respondResultError(w, "invalid result ID", http.StatusBadRequest)
		return
	}

	result, err := h.store.Get(r.Context(), id)
	if err != nil {
		msg, code := mapGetStoreError(err)
		respondResultError(w, msg, code)
		return
	}
	if result == nil {
		respondResultError(w, "result not found", http.StatusNotFound)
		return
	}

	respondResultJSON(w, result, http.StatusOK)
}

func respondResultError(w http.ResponseWriter, msg string, code int) {
	respondResultJSON(w, map[string]string{"error": msg}, code)
}

func respondResultJSON(w http.ResponseWriter, payload any, code int) {
	w.Header().Set(headerCacheControl, valueNoStore)
	respondJSON(w, payload, code)
}

func decodeSingleObject(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	if limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		_, _ = io.Copy(io.Discard, r.Body)
		return errTrailingJSON
	}
	return nil
}

// validResultID reports whether id is exactly eight ASCII letters or digits.
func validResultID(id string) bool {
	if len(id) != 8 {
		return false
	}
	for i := range len(id) {
		if !isAlnumByte(id[i]) {
			return false
		}
	}
	return true
}

func isAlnumByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func mapGetStoreError(err error) (string, int) {
	if errors.Is(err, results.ErrStoreRetryable) {
		return "store temporarily unavailable", http.StatusServiceUnavailable
	}
	return "internal error", http.StatusInternalServerError
}

func mapSaveStoreError(err error) (string, int) {
	if errors.Is(err, results.ErrStoreRetryable) {
		return "store temporarily unavailable", http.StatusServiceUnavailable
	}
	return "failed to save result", http.StatusInternalServerError
}

func hasNonFinite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return true
		}
	}
	return false
}
