package websocket

import (
	"bytes"
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

const (
	wsTypeConnected = "connected"
	wsTypeComplete  = "complete"
	wsTypeError     = "error"
	wsTypeMetrics   = "metrics"
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
	jsonBufPool    sync.Pool
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
		jsonBufPool: sync.Pool{
			New: func() any {
				return &bytes.Buffer{}
			},
		},
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

	if client.writeJSON(map[string]any{
		"type":      wsTypeConnected,
		"stream_id": streamID,
		"time":      time.Now().Unix(),
	}) != nil {
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
	isTerminal := isTerminalStreamStatus(state.Status)
	clientList, hasClients := s.snapshotClients(streamID)
	if !hasClients {
		s.cleanupTerminalStatus(streamID, isTerminal)
		return
	}
	msgType := websocketMessageType(state.Status)
	if isTerminal && !s.markTerminalOnce(streamID, state.Status) {
		return
	}

	msg := buildWebsocketMessage(streamID, state, msgType)

	buf, err := s.marshalMessage(msg)
	if err != nil {
		logging.Warn("WebSocket metrics marshal failed",
			logging.Field{Key: "stream_id", Value: streamID},
			logging.Field{Key: "error", Value: err})
		return
	}
	data := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
	s.broadcastToClients(streamID, clientList, data, isTerminal)
	s.jsonBufPool.Put(buf)

}

func (s *Server) snapshotClients(streamID string) ([]*clientConn, bool) {
	s.mu.RLock()
	clients := s.clients[streamID]
	if clients == nil {
		s.mu.RUnlock()
		return nil, false
	}
	clientList := make([]*clientConn, 0, len(clients))
	for _, client := range clients {
		clientList = append(clientList, client)
	}
	s.mu.RUnlock()
	return clientList, true
}

func (s *Server) cleanupTerminalStatus(streamID string, isTerminal bool) {
	if !isTerminal {
		return
	}
	s.mu.Lock()
	delete(s.sentStatus, streamID)
	s.mu.Unlock()
}

func websocketMessageType(status types.StreamStatus) string {
	switch status {
	case types.StreamStatusCompleted:
		return wsTypeComplete
	case types.StreamStatusFailed:
		return wsTypeError
	default:
		return wsTypeMetrics
	}
}

func (s *Server) markTerminalOnce(streamID string, status types.StreamStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sentStatus[streamID] == status {
		return false
	}
	s.sentStatus[streamID] = status
	return true
}

func buildWebsocketMessage(streamID string, state types.StreamSnapshot, msgType string) wsMessage {
	elapsed, remaining := streamElapsedAndRemaining(state)
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
		msg.Results = buildWebsocketResults(streamID, state)
	}
	if state.Status == types.StreamStatusFailed && state.Error != nil {
		msg.Error = state.Error.Error()
		msg.Message = state.Error.Error()
	}
	return msg
}

func streamElapsedAndRemaining(state types.StreamSnapshot) (float64, float64) {
	if state.StartTime.IsZero() {
		return 0, 0
	}
	elapsed := time.Since(state.StartTime).Seconds()
	if state.Config.Duration <= 0 {
		return elapsed, 0
	}
	remaining := state.Config.Duration.Seconds() - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return elapsed, remaining
}

func buildWebsocketResults(streamID string, state types.StreamSnapshot) *wsResults {
	return &wsResults{
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

func (s *Server) broadcastToClients(streamID string, clients []*clientConn, data []byte, isTerminal bool) {
	for _, client := range clients {
		if client.writeMessage(websocket.TextMessage, data) != nil {
			s.removeClient(streamID, client.conn)
			client.conn.Close()
			continue
		}
		if isTerminal {
			// Close terminal streams server-side so stale clients cannot leak forever.
			client.conn.Close()
		}
	}
}

func (s *Server) marshalMessage(msg wsMessage) (*bytes.Buffer, error) {
	buf, ok := s.jsonBufPool.Get().(*bytes.Buffer)
	if !ok {
		buf = &bytes.Buffer{}
	}
	buf.Reset()
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(msg); err != nil {
		s.jsonBufPool.Put(buf)
		return nil, err
	}
	return buf, nil
}

func (s *Server) startPingLoop() {
	s.wg.Go(func() {
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
	})
}

func (s *Server) Close() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.closeActiveConnections()
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
		if ref.client.writeMessage(websocket.PingMessage, nil) != nil {
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

func (s *Server) isAllowedOrigin(origin, host string) bool {
	s.mu.RLock()
	allowedOrigins := append([]string(nil), s.allowedOrigins...)
	s.mu.RUnlock()

	if origin == "" {
		return allowEmptyOrigin(allowedOrigins)
	}

	if len(allowedOrigins) == 0 {
		return sameOrigin(origin, host)
	}

	originHostValue := types.OriginHost(origin)
	for _, allowed := range allowedOrigins {
		if matchesAllowedOrigin(strings.TrimSpace(allowed), origin, originHostValue) {
			return true
		}
	}
	return false
}

func allowEmptyOrigin(allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return true
	}
	for _, allowed := range allowedOrigins {
		if strings.TrimSpace(allowed) == "*" {
			return true
		}
	}
	return false
}

func matchesAllowedOrigin(allowed, origin, originHostValue string) bool {
	if allowed == "" {
		return false
	}
	if allowed == "*" || strings.EqualFold(allowed, origin) {
		return true
	}
	if after, ok := strings.CutPrefix(allowed, "*."); ok {
		suffix := after
		return originHostValue != "" && (originHostValue == suffix || strings.HasSuffix(originHostValue, "."+suffix))
	}
	allowedHost := types.OriginHost(allowed)
	return allowedHost != "" && originHostValue != "" && strings.EqualFold(allowedHost, originHostValue)
}

func sameOrigin(origin, host string) bool {
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
	Metrics          types.Metrics `json:"metrics"`
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

func (c *clientConn) writeJSON(v any) error {
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

func (s *Server) closeActiveConnections() {
	s.mu.Lock()
	conns := make([]*websocket.Conn, 0)
	for streamID, streamClients := range s.clients {
		for conn := range streamClients {
			conns = append(conns, conn)
		}
		delete(s.clients, streamID)
	}
	s.sentStatus = make(map[string]types.StreamStatus)
	s.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func isTerminalStreamStatus(status types.StreamStatus) bool {
	return status == types.StreamStatusCompleted || status == types.StreamStatusFailed
}
