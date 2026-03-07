package api

import (
	stdErrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/saveenergy/openbyte/internal/config"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/internal/stream"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
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

type VersionResponse struct {
	Version string `json:"version"`
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
	StreamID      string `json:"stream_id"`
	WebSocketURL  string `json:"websocket_url"`
	TestServerTCP string `json:"test_server_tcp,omitempty"`
	TestServerUDP string `json:"test_server_udp,omitempty"`
	Status        string `json:"status"`
	Mode          string `json:"mode"`
}

type ServersResponse struct {
	Servers []types.ServerInfo `json:"servers"`
}

type streamConfigResponse struct {
	ID         string `json:"id"`
	Protocol   string `json:"protocol"`
	Direction  string `json:"direction"`
	Duration   int    `json:"duration"`
	Streams    int    `json:"streams"`
	PacketSize int    `json:"packet_size"`
	ClientIP   string `json:"client_ip,omitempty"`
}

type streamSnapshotResponse struct {
	Config    streamConfigResponse `json:"config"`
	Status    string               `json:"status"`
	Progress  float64              `json:"progress"`
	Metrics   types.Metrics        `json:"metrics"`
	Network   *types.NetworkInfo   `json:"network,omitempty"`
	StartTime time.Time            `json:"start_time"`
	EndTime   time.Time            `json:"end_time"`
	Error     string               `json:"error,omitempty"`
}

func (h *Handler) StartStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": contentTypeJSONErr}, http.StatusUnsupportedMediaType)
		return
	}

	req, mode, ok := decodeAndValidateStartRequest(w, r)
	if !ok {
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
		var streamErr *errors.StreamError
		if stdErrors.As(err, &streamErr) {
			if streamErr.Code == errors.ErrCodeStreamAlreadyExists {
				respondError(w, err, http.StatusConflict)
				return
			}
			if streamErr.Code == errors.ErrCodeResourceExhausted {
				respondError(w, err, http.StatusServiceUnavailable)
				return
			}
		}
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	if err := h.startCreatedStream(state.Config.ID); err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	resp := h.buildStartStreamResponse(r, state, mode)
	respondJSON(w, resp, http.StatusCreated)
}

func decodeAndValidateStartRequest(w http.ResponseWriter, r *http.Request) (StartStreamRequest, string, bool) {
	var req StartStreamRequest
	if err := decodeJSONBody(w, r, &req, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return StartStreamRequest{}, "", false
	}
	if req.Duration == 0 {
		req.Duration = defaultStreamDurationSec
	}
	if req.Streams == 0 {
		req.Streams = defaultStreamCount
	}
	mode := req.Mode
	if mode == "" {
		mode = "proxy"
	}
	if mode != "client" && mode != "proxy" {
		respondError(w, errors.ErrInvalidConfig("mode must be 'client' or 'proxy'", nil), http.StatusBadRequest)
		return StartStreamRequest{}, "", false
	}
	return req, mode, true
}

func (h *Handler) buildStartStreamResponse(r *http.Request, state *types.StreamState, mode string) StartStreamResponse {
	streamID := state.Config.ID
	resp := StartStreamResponse{
		StreamID:     streamID,
		WebSocketURL: "/api/v1/stream/" + streamID + "/stream",
		Status:       string(state.Status),
		Mode:         mode,
	}
	if mode == "client" && h.config != nil {
		host := responseHost(r, h.config)
		resp.TestServerTCP = host + ":" + strconv.Itoa(h.config.TCPTestPort)
		resp.TestServerUDP = host + ":" + strconv.Itoa(h.config.UDPTestPort)
	}
	return resp
}

func (h *Handler) startCreatedStream(streamID string) error {
	if err := h.manager.StartStream(streamID); err != nil {
		if cancelErr := h.manager.CancelStream(streamID); cancelErr != nil {
			logging.Warn("start stream cleanup failed",
				logging.Field{Key: "stream_id", Value: streamID},
				logging.Field{Key: "error", Value: cancelErr})
		}
		return err
	}
	return nil
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

func (h *Handler) ReportMetrics(w http.ResponseWriter, r *http.Request, streamID string) {
	if r.Method != http.MethodPost {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": contentTypeJSONErr}, http.StatusUnsupportedMediaType)
		return
	}

	var metrics types.Metrics
	if err := decodeJSONBody(w, r, &metrics, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return
	}
	if err := validateMetricsPayload(metrics); err != nil {
		respondError(w, err, http.StatusBadRequest)
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
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": methodNotAllowedErr}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": contentTypeJSONErr}, http.StatusUnsupportedMediaType)
		return
	}

	var req struct {
		Status  string        `json:"status"`
		Metrics types.Metrics `json:"metrics"`
		Error   string        `json:"error,omitempty"`
	}
	if err := decodeJSONBody(w, r, &req, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return
	}
	if err := validateMetricsPayload(req.Metrics); err != nil {
		respondError(w, err, http.StatusBadRequest)
		return
	}

	if req.Status == "completed" {
		if err := h.manager.CompleteStream(streamID, req.Metrics); err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
	} else if req.Status == "failed" {
		var cause error
		if reason := strings.TrimSpace(req.Error); reason != "" {
			cause = stdErrors.New(reason)
		}
		if err := h.manager.FailStreamWithError(streamID, req.Metrics, cause); err != nil {
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
	drainRequestBody(r)
	state, err := h.manager.GetStream(streamID)
	if err != nil {
		respondError(w, err, http.StatusNotFound)
		return
	}

	respondJSON(w, toStreamSnapshotResponse(state.GetState()), http.StatusOK)
}

func (h *Handler) GetStreamResults(w http.ResponseWriter, r *http.Request, streamID string) {
	drainRequestBody(r)
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

	respondJSON(w, toStreamSnapshotResponse(snapshot), http.StatusOK)
}

func toStreamSnapshotResponse(snapshot types.StreamSnapshot) streamSnapshotResponse {
	resp := streamSnapshotResponse{
		Config: streamConfigResponse{
			ID:         snapshot.Config.ID,
			Protocol:   string(snapshot.Config.Protocol),
			Direction:  string(snapshot.Config.Direction),
			Duration:   int(snapshot.Config.Duration.Seconds()),
			Streams:    snapshot.Config.Streams,
			PacketSize: snapshot.Config.PacketSize,
			ClientIP:   snapshot.Config.ClientIP,
		},
		Status:    string(snapshot.Status),
		Progress:  snapshot.Progress,
		Metrics:   snapshot.Metrics,
		Network:   snapshot.Network,
		StartTime: snapshot.StartTime,
		EndTime:   snapshot.EndTime,
	}
	if snapshot.Error != nil {
		resp.Error = snapshot.Error.Error()
	}
	return resp
}

func (h *Handler) CancelStream(w http.ResponseWriter, r *http.Request, streamID string) {
	drainRequestBody(r)
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

	maxDurationSec := 300
	if h.config != nil && h.config.MaxTestDuration > 0 {
		maxDurationSec = int(h.config.MaxTestDuration.Seconds())
	}
	if req.Duration < 1 || req.Duration > maxDurationSec {
		return types.StreamConfig{}, errors.ErrInvalidConfig(
			fmt.Sprintf("duration must be 1-%d seconds", maxDurationSec), nil)
	}

	maxStreams := 32
	if h.config != nil && h.config.MaxStreams > 0 {
		maxStreams = h.config.MaxStreams
	}
	if req.Streams < 1 || req.Streams > maxStreams {
		return types.StreamConfig{}, errors.ErrInvalidConfig(
			fmt.Sprintf("streams must be 1-%d", maxStreams), nil)
	}

	packetSize := req.PacketSize
	if packetSize <= 0 {
		packetSize = 1400
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
