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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logging.Warn("results: encode error response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		respondJSONError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxResultBodyBytes)

	decoder := json.NewDecoder(r.Body)
	var req saveRequest
	if err := decoder.Decode(&req); err != nil {
		io.Copy(io.Discard, r.Body)
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
		respondJSONError(w, "failed to save result", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(saveResponse{
		ID:  id,
		URL: "/results/" + id,
	}); err != nil {
		logging.Warn("results: encode save response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID.MatchString(id) {
		respondJSONError(w, "invalid result ID", http.StatusBadRequest)
		return
	}

	result, err := h.store.Get(id)
	if err != nil {
		respondJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if result == nil {
		respondJSONError(w, "result not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		logging.Warn("results: encode get response", logging.Field{Key: "error", Value: err})
	}
}

func hasNonFinite(vals ...float64) bool {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return true
		}
	}
	return false
}
