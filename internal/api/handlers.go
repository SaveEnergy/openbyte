package api

import "github.com/saveenergy/openbyte/internal/config"

type Handler struct {
	config           *config.Config
	clientIPResolver *ClientIPResolver
	version          string
}

func NewHandler() *Handler {
	return &Handler{}
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
