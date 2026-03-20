package api

import (
	stdErrors "errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/saveenergy/openbyte/internal/jsonbody"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/errors"
	"github.com/saveenergy/openbyte/pkg/types"
)

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
	if err := jsonbody.DecodeSingleObject(w, r, &req, maxJSONBodyBytes); err != nil {
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
