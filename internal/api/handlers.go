package api

import (
	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
)

type Handler struct {
	manager          *stream.Manager
	config           *config.Config
	clientIPResolver *ClientIPResolver
	version          string
}

const (
	maxJSONBodyBytes         = 1 << 20
	defaultStreamDurationSec = 30
	defaultStreamCount       = 4
	methodNotAllowedErr      = "method not allowed"
	contentTypeJSONErr       = "Content-Type must be application/json"
)

func NewHandler(manager *stream.Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

func (h *Handler) SetConfig(cfg *config.Config) {
	h.config = cfg
	h.clientIPResolver = NewClientIPResolver(cfg)
}

func (h *Handler) SetVersion(version string) {
	if version == "" {
		version = "dev"
	}
	h.version = version
}
