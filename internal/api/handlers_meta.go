package api

import (
	"net/http"
)

type VersionResponse struct {
	Version    string `json:"version"`
	ServerName string `json:"server_name"`
}

func (r *Router) GetVersion(w http.ResponseWriter, req *http.Request) {
	drainRequestBody(req)
	respondJSON(w, VersionResponse{Version: r.version, ServerName: r.serverName}, http.StatusOK)
}
