package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

type Handler struct {
	manager          *stream.Manager
	config           *config.Config
	clientIPResolver *ClientIPResolver
}

func NewHandler(manager *stream.Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

func (h *Handler) SetConfig(cfg *config.Config) {
	h.config = cfg
	h.clientIPResolver = NewClientIPResolver(cfg)
}

type StartStreamRequest struct {
	Protocol   string `json:"protocol"`
	Direction  string `json:"direction"`
	Duration   int    `json:"duration"`
	Streams    int    `json:"streams"`
	PacketSize int    `json:"packet_size,omitempty"`
	Mode       string `json:"mode,omitempty"`
}

type StartStreamResponse struct {
	StreamID       string `json:"stream_id"`
	WebSocketURL   string `json:"websocket_url"`
	TestServerTCP  string `json:"test_server_tcp,omitempty"`
	TestServerUDP  string `json:"test_server_udp,omitempty"`
	TestServerQUIC string `json:"test_server_quic,omitempty"`
	Status         string `json:"status"`
	Mode           string `json:"mode"`
}

type ServerInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	Region       string `json:"region,omitempty"`
	Host         string `json:"host"`
	TCPPort      int    `json:"tcp_port"`
	UDPPort      int    `json:"udp_port"`
	APIEndpoint  string `json:"api_endpoint"`
	Health       string `json:"health"`
	CapacityGbps int    `json:"capacity_gbps"`
	ActiveTests  int    `json:"active_tests"`
	MaxTests     int    `json:"max_tests"`
}

type ServersResponse struct {
	Servers []ServerInfo `json:"servers"`
}

func (h *Handler) StartStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "proxy"
	}
	if mode != "client" && mode != "proxy" {
		respondError(w, errors.ErrInvalidConfig("mode must be 'client' or 'proxy'", nil), http.StatusBadRequest)
		return
	}

	clientIP := h.resolveClientIP(r)
	config, err := h.validateConfig(req, clientIP)
	if err != nil {
		respondError(w, err, http.StatusBadRequest)
		return
	}

	state, err := h.manager.CreateStream(config)
	if err != nil {
		if streamErr, ok := err.(*errors.StreamError); ok && streamErr.Code == errors.ErrCodeStreamAlreadyExists {
			respondError(w, err, http.StatusConflict)
			return
		}
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	if mode == "proxy" {
		if err := h.manager.StartStream(state.Config.ID); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.manager.StartStream(state.Config.ID); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
	}

	streamID := state.Config.ID
	if streamID == "" {
		streamID = config.ID
	}
	wsURL := "/api/v1/stream/" + streamID + "/stream"

	resp := StartStreamResponse{
		StreamID:     streamID,
		WebSocketURL: wsURL,
		Status:       string(state.Status),
		Mode:         mode,
	}

	if mode == "client" && h.config != nil {
		host := r.Host
		if idx := len(host) - 1; idx > 0 {
			for i := len(host) - 1; i >= 0; i-- {
				if host[i] == ':' {
					host = host[:i]
					break
				}
			}
		}
		if host == "" || host == "localhost" {
			host = "127.0.0.1"
		}
		resp.TestServerTCP = host + ":" + strconv.Itoa(h.config.TCPTestPort)
		resp.TestServerUDP = host + ":" + strconv.Itoa(h.config.UDPTestPort)
		if h.config.QUICEnabled {
			resp.TestServerQUIC = host + ":" + strconv.Itoa(h.config.QUICPort)
		}
	}

	respondJSON(w, resp, http.StatusCreated)
}

func (h *Handler) GetServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	host := r.Host
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			host = host[:i]
			break
		}
	}
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}

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

	apiEndpoint := "http://" + host
	if h.config != nil && h.config.Port != "80" {
		apiEndpoint += ":" + h.config.Port
	}

	resp := ServersResponse{
		Servers: []ServerInfo{
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

func (h *Handler) ReportMetrics(w http.ResponseWriter, r *http.Request, streamID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metrics types.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.UpdateMetrics(streamID, metrics); err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}
	respondJSON(w, map[string]string{"status": "accepted"}, http.StatusAccepted)
}

func (h *Handler) CompleteStream(w http.ResponseWriter, r *http.Request, streamID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Status  string        `json:"status"`
		Metrics types.Metrics `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status == "completed" {
		if err := h.manager.CompleteStream(streamID, req.Metrics); err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
	} else if req.Status == "failed" {
		if err := h.manager.FailStream(streamID, req.Metrics); err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
	} else {
		respondError(w, errors.ErrInvalidConfig("status must be 'completed' or 'failed'", nil), http.StatusBadRequest)
		return
	}

	respondJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

func (h *Handler) GetStreamStatus(w http.ResponseWriter, r *http.Request, streamID string) {
	state, err := h.manager.GetStream(streamID)
	if err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	respondJSON(w, state.GetState(), http.StatusOK)
}

func (h *Handler) GetStreamResults(w http.ResponseWriter, r *http.Request, streamID string) {
	state, err := h.manager.GetStream(streamID)
	if err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	snapshot := state.GetState()
	if snapshot.Status != types.StreamStatusCompleted && snapshot.Status != types.StreamStatusFailed {
		respondJSON(w, map[string]string{
			"status": "stream not completed",
		}, http.StatusAccepted)
		return
	}

	respondJSON(w, snapshot, http.StatusOK)
}

func (h *Handler) CancelStream(w http.ResponseWriter, r *http.Request, streamID string) {
	if err := h.manager.CancelStream(streamID); err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{
		"status": "cancelled",
	}, http.StatusOK)
}

func (h *Handler) validateConfig(req StartStreamRequest, clientIP string) (types.StreamConfig, error) {
	var protocol types.Protocol
	switch req.Protocol {
	case "tcp":
		protocol = types.ProtocolTCP
	case "udp":
		protocol = types.ProtocolUDP
	case "quic":
		protocol = types.ProtocolQUIC
	default:
		return types.StreamConfig{}, errors.ErrInvalidConfig("invalid protocol", nil)
	}

	var direction types.Direction
	switch req.Direction {
	case "download":
		direction = types.DirectionDownload
	case "upload":
		direction = types.DirectionUpload
	case "bidirectional":
		direction = types.DirectionBidirectional
	default:
		return types.StreamConfig{}, errors.ErrInvalidConfig("invalid direction", nil)
	}

	if req.Duration < 1 || req.Duration > 300 {
		return types.StreamConfig{}, errors.ErrInvalidConfig("duration must be 1-300 seconds", nil)
	}

	if req.Streams < 1 || req.Streams > 16 {
		return types.StreamConfig{}, errors.ErrInvalidConfig("streams must be 1-16", nil)
	}

	packetSize := req.PacketSize
	if packetSize == 0 {
		packetSize = 1500
	}
	if packetSize < 64 || packetSize > 9000 {
		return types.StreamConfig{}, errors.ErrInvalidConfig("packet_size must be 64-9000 bytes", nil)
	}

	return types.StreamConfig{
		Protocol:   protocol,
		Direction:  direction,
		Duration:   time.Duration(req.Duration) * time.Second,
		Streams:    req.Streams,
		PacketSize: packetSize,
		StartTime:  time.Now(),
		ClientIP:   clientIP,
	}, nil
}

func (h *Handler) resolveClientIP(r *http.Request) string {
	if h.clientIPResolver == nil {
		return ipString(parseRemoteIP(r.RemoteAddr))
	}
	return h.clientIPResolver.FromRequest(r)
}

func respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, err error, statusCode int) {
	var msg string
	if streamErr, ok := err.(*errors.StreamError); ok {
		msg = streamErr.Message
	} else {
		msg = err.Error()
	}
	respondJSON(w, map[string]string{
		"error": msg,
	}, statusCode)
}
