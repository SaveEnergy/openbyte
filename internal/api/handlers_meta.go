package api

import (
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/config"
)

type VersionResponse struct {
	Version    string `json:"version"`
	ServerName string `json:"server_name"`
}

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	version := h.version
	if version == "" {
		version = "dev"
	}
	respondJSON(w, VersionResponse{Version: version, ServerName: h.serverDisplayName()}, http.StatusOK)
}

func (h *Handler) serverDisplayName() string {
	if h.config == nil {
		return config.DefaultServerName
	}
	name := strings.TrimSpace(h.config.ServerName)
	if name == "" {
		return config.DefaultServerName
	}
	return name
}
