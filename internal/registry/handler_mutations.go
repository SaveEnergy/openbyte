package registry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

func respondRegistryError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logging.Warn("registry: encode error response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) RegisterServer(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		respondRegistryError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if ct := r.Header.Get(headerContentType); ct != "" && !strings.HasPrefix(ct, contentTypeJSON) {
		respondRegistryError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRegistryBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var info types.ServerInfo
	if err := decoder.Decode(&info); err != nil {
		io.Copy(io.Discard, r.Body)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondRegistryError(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		respondRegistryError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		io.Copy(io.Discard, r.Body)
		respondRegistryError(w, "request body must contain a single JSON object", http.StatusBadRequest)
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

	w.Header().Set(headerContentType, contentTypeJSON)
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

	if ct := r.Header.Get(headerContentType); ct != "" && !strings.HasPrefix(ct, contentTypeJSON) {
		respondRegistryError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	id := r.PathValue("id")

	r.Body = http.MaxBytesReader(w, r.Body, maxRegistryBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req updateServerRequest
	if err := decoder.Decode(&req); err != nil {
		io.Copy(io.Discard, r.Body)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondRegistryError(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		respondRegistryError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		io.Copy(io.Discard, r.Body)
		respondRegistryError(w, "request body must contain a single JSON object", http.StatusBadRequest)
		return
	}

	if req.ID != nil && *req.ID != "" && *req.ID != id {
		respondRegistryError(w, "body ID conflicts with URL path", http.StatusBadRequest)
		return
	}

	if !h.service.UpdatePatched(id, func(dst *types.ServerInfo) {
		applyServerUpdatePatch(dst, req)
	}) {
		respondRegistryError(w, errServerNotFound, http.StatusNotFound)
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "updated",
		"server_id": id,
	}); err != nil {
		h.logger.Warn("encode update response", logging.Field{Key: "error", Value: err})
	}
}

func applyServerUpdatePatch(dst *types.ServerInfo, req updateServerRequest) {
	if req.Name != nil {
		dst.Name = *req.Name
	}
	if req.Location != nil {
		dst.Location = *req.Location
	}
	if req.Region != nil {
		dst.Region = *req.Region
	}
	if req.Host != nil {
		dst.Host = *req.Host
	}
	if req.TCPPort != nil {
		dst.TCPPort = *req.TCPPort
	}
	if req.UDPPort != nil {
		dst.UDPPort = *req.UDPPort
	}
	if req.APIEndpoint != nil {
		dst.APIEndpoint = *req.APIEndpoint
	}
	if req.Health != nil {
		dst.Health = *req.Health
	}
	if req.CapacityGbps != nil {
		dst.CapacityGbps = *req.CapacityGbps
	}
	if req.ActiveTests != nil {
		dst.ActiveTests = *req.ActiveTests
	}
	if req.MaxTests != nil {
		dst.MaxTests = *req.MaxTests
	}
}

func (h *Handler) DeregisterServer(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(r) {
		respondRegistryError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")

	if !h.service.Deregister(id) {
		respondRegistryError(w, errServerNotFound, http.StatusNotFound)
		return
	}

	h.logger.Info("Server deregistered", logging.Field{Key: "id", Value: id})

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":    "deregistered",
		"server_id": id,
	}); err != nil {
		h.logger.Warn("encode deregister response", logging.Field{Key: "error", Value: err})
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":  "healthy",
		"servers": h.service.Count(),
	}); err != nil {
		h.logger.Warn("encode health response", logging.Field{Key: "error", Value: err})
	}
}
