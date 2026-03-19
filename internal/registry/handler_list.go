package registry

import (
	"encoding/json"
	"net/http"

	"github.com/saveenergy/openbyte/internal/logging"
)

func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	healthy := r.URL.Query().Get("healthy") == "true"

	var servers []RegisteredServer
	if healthy {
		servers = h.service.ListHealthy()
	} else {
		servers = h.service.List()
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"servers": servers,
		"count":   len(servers),
	}); err != nil {
		h.logger.Warn("encode list response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) GetServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	server, exists := h.service.Get(id)
	if !exists {
		respondRegistryError(w, errServerNotFound, http.StatusNotFound)
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(server); err != nil {
		h.logger.Warn("encode server response", logging.Field{Key: "error", Value: err})
	}
}
