package websocket

import (
	"time"

	"github.com/gorilla/websocket"
)

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
