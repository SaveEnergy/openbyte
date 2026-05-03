package api

import (
	"net/http"
)

type VersionResponse struct {
	Version string `json:"version"`
}

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	version := h.version
	if version == "" {
		version = "dev"
	}
	respondJSON(w, VersionResponse{Version: version}, http.StatusOK)
}
