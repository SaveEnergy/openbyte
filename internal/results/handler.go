package results

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/saveenergy/openbyte/internal/logging"
)

var validID = regexp.MustCompile(`^[0-9a-zA-Z]{8}$`)

const maxResultBodyBytes = 4096

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

type saveRequest struct {
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

type saveResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func respondJSONError(w http.ResponseWriter, msg string, code int) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		logging.Warn("results: marshal response failed", logging.Field{Key: "error", Value: err})
		code = http.StatusInternalServerError
		body = []byte(`{"error":"internal error"}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json")
	if code == http.StatusOK {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(code)
	if _, err := w.Write(body); err != nil {
		logging.Warn("results: write response failed", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		drainRequestBody(r)
		respondJSONError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxResultBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req saveRequest
	if err := decoder.Decode(&req); err != nil {
		io.Copy(io.Discard, r.Body)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondJSONError(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		respondJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		io.Copy(io.Discard, r.Body)
		respondJSONError(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return
	}

	if req.DownloadMbps < 0 || req.UploadMbps < 0 || req.LatencyMs < 0 ||
		req.JitterMs < 0 || req.LoadedLatencyMs < 0 {
		respondJSONError(w, "numeric fields must be >= 0", http.StatusBadRequest)
		return
	}
	if hasNonFinite(req.DownloadMbps, req.UploadMbps, req.LatencyMs, req.JitterMs, req.LoadedLatencyMs) {
		respondJSONError(w, "numeric fields must be finite", http.StatusBadRequest)
		return
	}
	if req.DownloadMbps > 100000 || req.UploadMbps > 100000 ||
		req.LatencyMs > 60000 || req.JitterMs > 60000 || req.LoadedLatencyMs > 60000 {
		respondJSONError(w, "values out of reasonable range", http.StatusBadRequest)
		return
	}
	if len(req.ServerName) > 200 || len(req.IPv4) > 45 || len(req.IPv6) > 45 ||
		len(req.BufferbloatGrade) > 5 {
		respondJSONError(w, "field too long", http.StatusBadRequest)
		return
	}

	result := Result{
		DownloadMbps:     req.DownloadMbps,
		UploadMbps:       req.UploadMbps,
		LatencyMs:        req.LatencyMs,
		JitterMs:         req.JitterMs,
		LoadedLatencyMs:  req.LoadedLatencyMs,
		BufferbloatGrade: req.BufferbloatGrade,
		IPv4:             req.IPv4,
		IPv6:             req.IPv6,
		ServerName:       req.ServerName,
	}

	id, err := h.store.Save(result)
	if err != nil {
		logging.Warn("results: save failed", logging.Field{Key: "error", Value: err})
		msg, code := mapSaveStoreError(err)
		respondJSONError(w, msg, code)
		return
	}

	writeJSON(w, http.StatusCreated, saveResponse{
		ID:  id,
		URL: "/results/" + id,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID.MatchString(id) {
		respondJSONError(w, "invalid result ID", http.StatusBadRequest)
		return
	}

	result, err := h.store.Get(id)
	if err != nil {
		msg, code := mapGetStoreError(err)
		respondJSONError(w, msg, code)
		return
	}
	if result == nil {
		respondJSONError(w, "result not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func mapGetStoreError(err error) (string, int) {
	if errors.Is(err, ErrStoreRetryable) {
		return "store temporarily unavailable", http.StatusServiceUnavailable
	}
	return "internal error", http.StatusInternalServerError
}

func mapSaveStoreError(err error) (string, int) {
	if errors.Is(err, ErrStoreRetryable) {
		return "store temporarily unavailable", http.StatusServiceUnavailable
	}
	return "failed to save result", http.StatusInternalServerError
}

func hasNonFinite(vals ...float64) bool {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return true
		}
	}
	return false
}

func drainRequestBody(r *http.Request) {
	if r == nil || r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
}
