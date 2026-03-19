package api

import (
	"net/http"

	"github.com/saveenergy/openbyte/pkg/types"
)

type VersionResponse struct {
	Version string `json:"version"`
}

type ServersResponse struct {
	Servers []types.ServerInfo `json:"servers"`
}

func (h *Handler) GetServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}

	host := responseHost(r, h.config)

	serverID := "default"
	serverName := "OpenByte Server"
	serverLocation := host
	serverRegion := ""
	tcpPort := 8081
	udpPort := 8082
	capacityGbps := 25
	maxTests := 10
	activeTests := 0

	if h.config != nil {
		serverID = h.config.ServerID
		serverName = h.config.ServerName
		serverLocation = h.config.ServerLocation
		serverRegion = h.config.ServerRegion
		tcpPort = h.config.TCPTestPort
		udpPort = h.config.UDPTestPort
		capacityGbps = h.config.CapacityGbps
		maxTests = h.config.MaxConcurrentTests

		if h.config.PublicHost != "" {
			host = h.config.PublicHost
		}
	}

	if h.manager != nil {
		activeTests = h.manager.ActiveCount()
	}

	scheme := requestScheme(r, h.config)
	hostForEndpoint := responseHostForEndpoint(r, h.config)
	apiEndpoint := scheme + "://" + hostForEndpoint

	resp := ServersResponse{
		Servers: []types.ServerInfo{
			{
				ID:           serverID,
				Name:         serverName,
				Location:     serverLocation,
				Region:       serverRegion,
				Host:         host,
				TCPPort:      tcpPort,
				UDPPort:      udpPort,
				APIEndpoint:  apiEndpoint,
				Health:       "healthy",
				CapacityGbps: capacityGbps,
				ActiveTests:  activeTests,
				MaxTests:     maxTests,
			},
		},
	}

	respondJSON(w, resp, http.StatusOK)
}

func (h *Handler) GetVersion(w http.ResponseWriter, r *http.Request) {
	drainRequestBody(r)
	version := h.version
	if version == "" {
		version = "dev"
	}
	respondJSON(w, VersionResponse{Version: version}, http.StatusOK)
}
