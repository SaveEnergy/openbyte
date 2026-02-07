package websocket

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

type Server struct {
	upgrader       websocket.Upgrader
	clients        map[string]map[*websocket.Conn]*clientConn
	sentStatus     map[string]types.StreamStatus
	allowedOrigins []string
	pingInterval   time.Duration
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
	mu             sync.RWMutex
}

type clientConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewServer() *Server {
	server := &Server{
		clients:      make(map[string]map[*websocket.Conn]*clientConn),
		sentStatus:   make(map[string]types.StreamStatus),
		pingInterval: 30 * time.Second,
		stopCh:       make(chan struct{}),
	}
	server.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return server.isAllowedOrigin(r.Header.Get("Origin"), r.Host)
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	server.startPingLoop()
	return server
}

func (s *Server) SetAllowedOrigins(origins []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedOrigins = origins
}

func (s *Server) SetPingInterval(interval time.Duration) {
	if interval <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pingInterval = interval
}

func (s *Server) HandleStream(w http.ResponseWriter, r *http.Request, streamID string) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Error("WebSocket upgrade error",
			logging.Field{Key: "error", Value: err},
			logging.Field{Key: "stream_id", Value: streamID})
		return
	}
	defer conn.Close()

	// Server only reads for disconnect detection — limit frame size to prevent memory abuse.
	conn.SetReadLimit(4096)

	s.mu.Lock()
	if s.clients[streamID] == nil {
		s.clients[streamID] = make(map[*websocket.Conn]*clientConn)
	}
	client := &clientConn{conn: conn}
	s.clients[streamID][conn] = client
	s.mu.Unlock()

	if err := client.writeJSON(map[string]interface{}{
		"type":      "connected",
		"stream_id": streamID,
		"time":      time.Now().Unix(),
	}); err != nil {
		s.removeClient(streamID, conn)
		return
	}

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	s.removeClient(streamID, conn)
}

func (s *Server) BroadcastMetrics(streamID string, state types.StreamSnapshot) {
	s.mu.RLock()
	clients := s.clients[streamID]
	if clients == nil {
		s.mu.RUnlock()
		// Clean up sentStatus for streams with no clients on terminal status
		if state.Status == types.StreamStatusCompleted || state.Status == types.StreamStatusFailed {
			s.mu.Lock()
			delete(s.sentStatus, streamID)
			s.mu.Unlock()
		}
		return
	}

	clientList := make([]*clientConn, 0, len(clients))
	for _, client := range clients {
		clientList = append(clientList, client)
	}
	lastSentStatus := s.sentStatus[streamID]
	s.mu.RUnlock()

	var msgType string
	switch state.Status {
	case types.StreamStatusCompleted:
		msgType = "complete"
	case types.StreamStatusFailed:
		msgType = "error"
	default:
		msgType = "metrics"
	}

	if (state.Status == types.StreamStatusCompleted || state.Status == types.StreamStatusFailed) &&
		lastSentStatus == state.Status {
		return
	}

	elapsed := float64(0)
	remaining := float64(0)
	if !state.StartTime.IsZero() {
		elapsed = time.Since(state.StartTime).Seconds()
		if state.Config.Duration > 0 {
			remaining = state.Config.Duration.Seconds() - elapsed
			if remaining < 0 {
				remaining = 0
			}
		}
	}

	msg := wsMessage{
		Type:             msgType,
		StreamID:         streamID,
		Status:           string(state.Status),
		Progress:         state.Progress,
		ElapsedSeconds:   elapsed,
		RemainingSeconds: remaining,
		Metrics:          state.Metrics,
		Time:             time.Now().Unix(),
	}

	if state.Status == types.StreamStatusCompleted {
		msg.Results = &wsResults{
			StreamID: streamID,
			Status:   string(state.Status),
			Config: wsResultConfig{
				Protocol:   string(state.Config.Protocol),
				Direction:  string(state.Config.Direction),
				Duration:   int(state.Config.Duration.Seconds()),
				Streams:    state.Config.Streams,
				PacketSize: state.Config.PacketSize,
			},
			Results: wsResultMetrics{
				ThroughputMbps:    state.Metrics.ThroughputMbps,
				ThroughputAvgMbps: state.Metrics.ThroughputAvgMbps,
				Latency:           state.Metrics.Latency,
				JitterMs:          state.Metrics.JitterMs,
				PacketLossPercent: state.Metrics.PacketLossPercent,
				BytesTransferred:  state.Metrics.BytesTransferred,
				PacketsSent:       state.Metrics.PacketsSent,
				PacketsReceived:   state.Metrics.PacketsReceived,
			},
			StartTime:       state.StartTime.Format("2006-01-02T15:04:05Z07:00"),
			EndTime:         time.Now().Format("2006-01-02T15:04:05Z07:00"),
			DurationSeconds: time.Since(state.StartTime).Seconds(),
		}
	}

	if state.Status == types.StreamStatusFailed && state.Error != nil {
		msg.Error = state.Error.Error()
		msg.Message = state.Error.Error()
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logging.Warn("WebSocket metrics marshal failed",
			logging.Field{Key: "stream_id", Value: streamID},
			logging.Field{Key: "error", Value: err})
		return
	}

	for _, client := range clientList {
		if err := client.writeMessage(websocket.TextMessage, data); err != nil {
			s.removeClient(streamID, client.conn)
			client.conn.Close()
		}
	}

	if state.Status == types.StreamStatusCompleted || state.Status == types.StreamStatusFailed {
		s.mu.Lock()
		s.sentStatus[streamID] = state.Status
		s.mu.Unlock()
	}
}

func (s *Server) startPingLoop() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		interval := s.getPingInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.pingClients()
				next := s.getPingInterval()
				if next != interval {
					ticker.Stop()
					interval = next
					ticker = time.NewTicker(interval)
				}
			}
		}
	}()
}

func (s *Server) Close() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *Server) getPingInterval() time.Duration {
	s.mu.RLock()
	interval := s.pingInterval
	s.mu.RUnlock()
	if interval <= 0 {
		return 30 * time.Second
	}
	return interval
}

func (s *Server) pingClients() {
	type clientRef struct {
		streamID string
		client   *clientConn
	}

	var refs []clientRef
	s.mu.RLock()
	for streamID, streamClients := range s.clients {
		for _, client := range streamClients {
			refs = append(refs, clientRef{streamID: streamID, client: client})
		}
	}
	s.mu.RUnlock()

	for _, ref := range refs {
		if err := ref.client.writeMessage(websocket.PingMessage, nil); err != nil {
			s.removeClient(ref.streamID, ref.client.conn)
			ref.client.conn.Close()
		}
	}
}

func (s *Server) removeClient(streamID string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clients[streamID] == nil {
		return
	}
	delete(s.clients[streamID], conn)
	if len(s.clients[streamID]) == 0 {
		delete(s.clients, streamID)
		delete(s.sentStatus, streamID)
	}
}

func (s *Server) isAllowedOrigin(origin string, host string) bool {
	if origin == "" {
		return true
	}

	s.mu.RLock()
	allowedOrigins := append([]string(nil), s.allowedOrigins...)
	s.mu.RUnlock()

	if len(allowedOrigins) == 0 {
		return sameOrigin(origin, host)
	}

	originHostValue := types.OriginHost(origin)
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, origin) {
			return true
		}
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*.")
			if originHostValue != "" && (originHostValue == suffix || strings.HasSuffix(originHostValue, "."+suffix)) {
				return true
			}
		}
		allowedHost := types.OriginHost(allowed)
		if allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue) {
			return true
		}
	}
	return false
}

func sameOrigin(origin string, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originH := types.StripHostPort(parsed.Host)
	requestH := types.StripHostPort(host)
	return strings.EqualFold(originH, requestH)
}

type wsMessage struct {
	Type             string        `json:"type"`
	StreamID         string        `json:"stream_id"`
	Status           string        `json:"status"`
	Progress         float64       `json:"progress,omitempty"`
	ElapsedSeconds   float64       `json:"elapsed_seconds,omitempty"`
	RemainingSeconds float64       `json:"remaining_seconds,omitempty"`
	Metrics          types.Metrics `json:"metrics,omitempty"`
	Time             int64         `json:"time"`
	Results          *wsResults    `json:"results,omitempty"`
	Error            string        `json:"error,omitempty"`
	Message          string        `json:"message,omitempty"`
}

type wsResults struct {
	StreamID        string          `json:"stream_id"`
	Status          string          `json:"status"`
	Config          wsResultConfig  `json:"config"`
	Results         wsResultMetrics `json:"results"`
	StartTime       string          `json:"start_time"`
	EndTime         string          `json:"end_time"`
	DurationSeconds float64         `json:"duration_seconds"`
}

type wsResultConfig struct {
	Protocol   string `json:"protocol"`
	Direction  string `json:"direction"`
	Duration   int    `json:"duration"`
	Streams    int    `json:"streams"`
	PacketSize int    `json:"packet_size"`
}

type wsResultMetrics struct {
	ThroughputMbps    float64              `json:"throughput_mbps"`
	ThroughputAvgMbps float64              `json:"throughput_avg_mbps"`
	Latency           types.LatencyMetrics `json:"latency_ms"`
	JitterMs          float64              `json:"jitter_ms"`
	PacketLossPercent float64              `json:"packet_loss_percent"`
	BytesTransferred  int64                `json:"bytes_transferred"`
	PacketsSent       int64                `json:"packets_sent"`
	PacketsReceived   int64                `json:"packets_received"`
}

func (c *clientConn) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(v)
}

func (c *clientConn) writeMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteMessage(messageType, data)
}
