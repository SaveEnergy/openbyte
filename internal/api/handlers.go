package api

import (
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"io"
	"math"
	"net"
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
	StartTime time.Time            `json:"start_time,omitempty"`
	EndTime   time.Time            `json:"end_time,omitempty"`
	Error     string               `json:"error,omitempty"`
}

func (h *Handler) StartStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": "Content-Type must be application/json"}, http.StatusUnsupportedMediaType)
		return
	}

	var req StartStreamRequest
	if err := decodeJSONBody(w, r, &req, maxJSONBodyBytes); err != nil {
		respondJSONBodyError(w, err)
		return
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

	streamID := state.Config.ID
	wsURL := "/api/v1/stream/" + streamID + "/stream"

	resp := StartStreamResponse{
		StreamID:     streamID,
		WebSocketURL: wsURL,
		Status:       string(state.Status),
		Mode:         mode,
	}

	if mode == "client" && h.config != nil {
		host := responseHost(r, h.config)
		resp.TestServerTCP = host + ":" + strconv.Itoa(h.config.TCPTestPort)
		resp.TestServerUDP = host + ":" + strconv.Itoa(h.config.UDPTestPort)
	}

	respondJSON(w, resp, http.StatusCreated)
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
		respondJSON(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
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
	isProxied := (h.config != nil && h.config.TrustProxyHeaders &&
		(r.Header.Get("X-Forwarded-Proto") != "" || r.Header.Get("X-Forwarded-For") != "")) ||
		(h.config != nil && h.config.PublicHost != "")
	apiEndpoint := scheme + "://" + host
	if h.config != nil && !isProxied {
		if (scheme == "http" && h.config.Port != "80") || (scheme == "https" && h.config.Port != "443") {
			apiEndpoint += ":" + h.config.Port
		}
	}

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
		respondJSON(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": "Content-Type must be application/json"}, http.StatusUnsupportedMediaType)
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
		respondJSON(w, map[string]string{"error": "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	if !isJSONContentType(r) {
		drainRequestBody(r)
		respondJSON(w, map[string]string{"error": "Content-Type must be application/json"}, http.StatusUnsupportedMediaType)
		return
	}

	var req struct {
		Status  string        `json:"status"`
		Metrics types.Metrics `json:"metrics"`
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

func normalizeHost(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	trimmed := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		trimmed = h
		if strings.Contains(h, ":") && strings.Contains(host, "[") {
			trimmed = "[" + h + "]"
		}
	}
	if trimmed == "" || trimmed == "localhost" {
		return "127.0.0.1"
	}
	return trimmed
}

func requestScheme(r *http.Request, cfg *config.Config) string {
	if r == nil {
		return "http"
	}
	if cfg != nil && cfg.TrustProxyHeaders {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			if strings.EqualFold(proto, "https") {
				return "https"
			}
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func responseHost(r *http.Request, cfg *config.Config) string {
	if cfg != nil {
		if cfg.PublicHost != "" {
			return cfg.PublicHost
		}
		if !cfg.TrustProxyHeaders {
			return normalizeHost(cfg.BindAddress)
		}
	}
	return normalizeHost(r.Host)
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}, limit int64) error {
	if limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		io.Copy(io.Discard, r.Body)
		return err
	}
	if err := decoder.Decode(&struct{}{}); !stdErrors.Is(err, io.EOF) {
		io.Copy(io.Discard, r.Body)
		return stdErrors.New("request body must contain a single JSON object")
	}
	return nil
}

func isJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json")
}

func validateMetricsPayload(m types.Metrics) error {
	values := []float64{
		m.ThroughputMbps, m.ThroughputAvgMbps, m.JitterMs, m.PacketLossPercent,
		m.Latency.MinMs, m.Latency.MaxMs, m.Latency.AvgMs, m.Latency.P50Ms, m.Latency.P95Ms, m.Latency.P99Ms,
	}
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errors.ErrInvalidConfig("metrics contain non-finite values", nil)
		}
	}
	if m.ThroughputMbps < 0 || m.ThroughputAvgMbps < 0 {
		return errors.ErrInvalidConfig("metrics throughput must be >= 0", nil)
	}
	if m.BytesTransferred < 0 {
		return errors.ErrInvalidConfig("metrics bytes_transferred must be >= 0", nil)
	}
	if m.JitterMs < 0 {
		return errors.ErrInvalidConfig("metrics jitter_ms must be >= 0", nil)
	}
	if m.PacketLossPercent < 0 || m.PacketLossPercent > 100 {
		return errors.ErrInvalidConfig("metrics packet_loss_percent must be between 0 and 100", nil)
	}
	if m.Latency.MinMs < 0 || m.Latency.MaxMs < 0 || m.Latency.AvgMs < 0 || m.Latency.P50Ms < 0 || m.Latency.P95Ms < 0 || m.Latency.P99Ms < 0 {
		return errors.ErrInvalidConfig("metrics latency values must be >= 0", nil)
	}
	if m.Latency.Count < 0 {
		return errors.ErrInvalidConfig("metrics latency count must be >= 0", nil)
	}
	return nil
}

func drainRequestBody(r *http.Request) {
	if r == nil || r.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()
}

func respondJSONBodyError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if stdErrors.As(err, &maxErr) {
		respondJSON(w, map[string]string{"error": "request body too large"}, http.StatusRequestEntityTooLarge)
		return
	}
	respondJSON(w, map[string]string{"error": "invalid request body"}, http.StatusBadRequest)
}

func respondJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	payload, err := json.Marshal(data)
	if err != nil {
		logging.Warn("JSON response marshal failed",
			logging.Field{Key: "error", Value: err})
		statusCode = http.StatusInternalServerError
		payload = []byte(`{"error":"internal error"}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(payload); err != nil {
		logging.Warn("JSON response write failed",
			logging.Field{Key: "error", Value: err})
	}
}

func respondError(w http.ResponseWriter, err error, statusCode int) {
	var msg string
	var streamErr *errors.StreamError
	if stdErrors.As(err, &streamErr) {
		msg = streamErr.Message
	} else {
		msg = err.Error()
	}
	respondJSON(w, map[string]string{
		"error": msg,
	}, statusCode)
}
