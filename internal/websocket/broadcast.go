package websocket

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

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

func isTerminalStreamStatus(status types.StreamStatus) bool {
	return status == types.StreamStatusCompleted || status == types.StreamStatusFailed
}
