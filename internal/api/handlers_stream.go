package api

import (
	stdErrors "errors"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/jsonbody"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

func (h *Handler) ReportMetrics(w http.ResponseWriter, r *http.Request, streamID string) {
	if r.Method != http.MethodPost {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": contentTypeJSONErr}, http.StatusUnsupportedMediaType)
		return
	}

	var metrics types.Metrics
	if err := jsonbody.DecodeSingleObject(w, r, &metrics, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return
	}
	if err := validateMetricsPayload(metrics); err != nil {
		respondError(w, err, http.StatusBadRequest)
		return
	}

	if err := h.manager.UpdateMetrics(streamID, metrics); err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}
	respondJSON(w, map[string]string{"status": "accepted"}, http.StatusAccepted)
}

func (h *Handler) CompleteStream(w http.ResponseWriter, r *http.Request, streamID string) {
	if r.Method != http.MethodPost {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": contentTypeJSONErr}, http.StatusUnsupportedMediaType)
		return
	}

	var req struct {
		Status  string        `json:"status"`
		Metrics types.Metrics `json:"metrics"`
		Error   string        `json:"error,omitempty"`
	}
	if err := jsonbody.DecodeSingleObject(w, r, &req, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return
	}
	if err := validateMetricsPayload(req.Metrics); err != nil {
		respondError(w, err, http.StatusBadRequest)
		return
	}

	if req.Status == "completed" {
		if err := h.manager.CompleteStream(streamID, req.Metrics); err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
	} else if req.Status == "failed" {
		var cause error
		if reason := strings.TrimSpace(req.Error); reason != "" {
			cause = stdErrors.New(reason)
		}
		if err := h.manager.FailStreamWithError(streamID, req.Metrics, cause); err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
	} else {
		respondError(w, errors.ErrInvalidConfig("status must be 'completed' or 'failed'", nil), http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (h *Handler) GetStreamStatus(w http.ResponseWriter, r *http.Request, streamID string) {
	drainRequestBody(r)
	state, err := h.manager.GetStream(streamID)
	if err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	respondJSON(w, toStreamSnapshotResponse(state.GetState()), http.StatusOK)
}

func (h *Handler) GetStreamResults(w http.ResponseWriter, r *http.Request, streamID string) {
	drainRequestBody(r)
	state, err := h.manager.GetStream(streamID)
	if err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	snapshot := state.GetState()
	if snapshot.Status != types.StreamStatusCompleted && snapshot.Status != types.StreamStatusFailed {
		respondJSON(w, map[string]string{
			"status": "stream not completed",
		}, http.StatusAccepted)
		return
	}

	respondJSON(w, toStreamSnapshotResponse(snapshot), http.StatusOK)
}

func (h *Handler) CancelStream(w http.ResponseWriter, r *http.Request, streamID string) {
	drainRequestBody(r)
	if err := h.manager.CancelStream(streamID); err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{
		"status": "cancelled",
	}, http.StatusOK)
}
