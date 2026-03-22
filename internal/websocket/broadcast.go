package websocket

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/internal/logging"
	"github.com/saveenergy/openbyte/pkg/types"
)

// wsMarshalState pools a [json.Encoder] wired to an internal [bytes.Buffer] so
// BroadcastMetrics avoids allocating a new encoder per marshal.
type wsMarshalState struct {
	buf bytes.Buffer
	enc *json.Encoder
}

func newWSMarshalState() *wsMarshalState {
	st := &wsMarshalState{}
	st.enc = json.NewEncoder(&st.buf)
	st.enc.SetEscapeHTML(false)
	return st
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

	st, err := s.marshalMessage(msg)
	if err != nil {
		logging.Warn("WebSocket metrics marshal failed",
			logging.Field{Key: "stream_id", Value: streamID},
			logging.Field{Key: "error", Value: err})
		return
	}
	data := st.buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	s.broadcastToClients(streamID, clientList, data, isTerminal)
	s.wsMarshalPool.Put(st)

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

func (s *Server) marshalMessage(msg wsMessage) (*wsMarshalState, error) {
	st, ok := s.wsMarshalPool.Get().(*wsMarshalState)
	if !ok {
		st = newWSMarshalState()
	}
	st.buf.Reset()
	// Metrics/completion payloads are larger than tiny HTTP JSON; reduce encoder
	// reallocations when the pooled buffer returns with low capacity after Reset.
	st.buf.Grow(2048)
	if err := st.enc.Encode(msg); err != nil {
		s.wsMarshalPool.Put(st)
		return nil, err
	}
	return st, nil
}

func isTerminalStreamStatus(status types.StreamStatus) bool {
	return status == types.StreamStatusCompleted || status == types.StreamStatusFailed
}
