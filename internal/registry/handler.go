package registry

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/saveenergy/openbyte/internal/logging"
)

type Handler struct {
	service     *Service
	logger      *logging.Logger
	apiKey      string
	apiKeyBytes []byte // []byte(apiKey) for ConstantTimeCompare without per-request conversion
}

const (
	authBearerPrefix  = "Bearer "
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
	errServerNotFound = "server not found"
)

// maxRegistryBodySize limits JSON request bodies for registry endpoints.
const maxRegistryBodySize = 1024 * 64 // 64 KB

type updateServerRequest struct {
	ID           *string `json:"id,omitempty"`
	Name         *string `json:"name,omitempty"`
	Location     *string `json:"location,omitempty"`
	Region       *string `json:"region,omitempty"`
	Host         *string `json:"host,omitempty"`
	TCPPort      *int    `json:"tcp_port,omitempty"`
	UDPPort      *int    `json:"udp_port,omitempty"`
	APIEndpoint  *string `json:"api_endpoint,omitempty"`
	Health       *string `json:"health,omitempty"`
	CapacityGbps *int    `json:"capacity_gbps,omitempty"`
	ActiveTests  *int    `json:"active_tests,omitempty"`
	MaxTests     *int    `json:"max_tests,omitempty"`
}

func NewHandler(service *Service, logger *logging.Logger, apiKey string) *Handler {
	return &Handler{
		service:     service,
		logger:      logger,
		apiKey:      apiKey,
		apiKeyBytes: []byte(apiKey),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/registry/servers", h.ListServers)
	mux.HandleFunc("POST /api/v1/registry/servers", h.RegisterServer)
	mux.HandleFunc("GET /api/v1/registry/servers/{id}", h.GetServer)
	mux.HandleFunc("PUT /api/v1/registry/servers/{id}", h.UpdateServer)
	mux.HandleFunc("DELETE /api/v1/registry/servers/{id}", h.DeregisterServer)
	mux.HandleFunc("GET /api/v1/registry/health", h.Health)
}

func (h *Handler) authenticate(r *http.Request) bool {
	if h.apiKey == "" {
		return true
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	if !strings.HasPrefix(auth, authBearerPrefix) {
		return false
	}
	token := auth[len(authBearerPrefix):]
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), h.apiKeyBytes) == 1
}
