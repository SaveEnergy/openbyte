package registry

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/saveenergy/openbyte/internal/logging"
)

type Handler struct {
	service *Service
	logger  *logging.Logger
	apiKey  string
}

func NewHandler(service *Service, logger *logging.Logger, apiKey string) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
		apiKey:  apiKey,
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	registry := r.PathPrefix("/api/v1/registry").Subrouter()

	registry.HandleFunc("/servers", h.ListServers).Methods(http.MethodGet)
	registry.HandleFunc("/servers", h.RegisterServer).Methods(http.MethodPost)
	registry.HandleFunc("/servers/{id}", h.GetServer).Methods(http.MethodGet)
	registry.HandleFunc("/servers/{id}", h.UpdateServer).Methods(http.MethodPut)
	registry.HandleFunc("/servers/{id}", h.DeregisterServer).Methods(http.MethodDelete)
	registry.HandleFunc("/health", h.Health).Methods(http.MethodGet)
}

func (h *Handler) authenticate(r *http.Request) bool {
	if h.apiKey == "" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	if len(auth) > 7 && auth[:7] == "Bearer " {
		return subtle.ConstantTimeCompare([]byte(auth[7:]), []byte(h.apiKey)) == 1
	}
	return false
}

// maxRegistryBodySize limits JSON request bodies for registry endpoints.
const maxRegistryBodySize = 1024 * 64 // 64 KB

func (h *Handler) ListServers(w http.ResponseWriter, r *http.Request) {
	healthy := r.URL.Query().Get("healthy") == "true"

	var servers []RegisteredServer
	if healthy {
		servers = h.service.ListHealthy()
	} else {
		servers = h.service.List()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	}); err != nil {
		h.logger.Warn("encode list response", logging.Field{Key: "error", Value: err})
	}
}

func respondRegistryError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logging.Warn("registry: encode error response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) GetServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	server, exists := h.service.Get(id)
	if !exists {
		respondRegistryError(w, "server not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(server); err != nil {
		h.logger.Warn("encode server response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) RegisterServer(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		respondRegistryError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRegistryBodySize)
	var info ServerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		respondRegistryError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if info.ID == "" {
		respondRegistryError(w, "server ID is required", http.StatusBadRequest)
		return
	}

	h.service.Register(info)
	h.logger.Info("Server registered",
		logging.Field{Key: "id", Value: info.ID},
		logging.Field{Key: "name", Value: info.Name})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "registered",
		"server_id": info.ID,
	}); err != nil {
		h.logger.Warn("encode register response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		respondRegistryError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	r.Body = http.MaxBytesReader(w, r.Body, maxRegistryBodySize)
	var info ServerInfo
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		respondRegistryError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	info.ID = id

	if !h.service.Update(id, info) {
		respondRegistryError(w, "server not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "updated",
		"server_id": id,
	}); err != nil {
		h.logger.Warn("encode update response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) DeregisterServer(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		respondRegistryError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if !h.service.Deregister(id) {
		respondRegistryError(w, "server not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Server deregistered", logging.Field{Key: "id", Value: id})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "deregistered",
		"server_id": id,
	}); err != nil {
		h.logger.Warn("encode deregister response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"servers": h.service.Count(),
	}); err != nil {
		h.logger.Warn("encode health response", logging.Field{Key: "error", Value: err})
	}
}
