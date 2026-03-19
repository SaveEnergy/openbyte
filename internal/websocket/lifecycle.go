package websocket

import (
	"github.com/gorilla/websocket"
	"github.com/saveenergy/openbyte/pkg/types"
)

func (s *Server) Close() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.closeActiveConnections()
	s.wg.Wait()
}

func (s *Server) removeClient(streamID string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clients[streamID] == nil {
		return
	}
	client := s.clients[streamID][conn]
	delete(s.clients[streamID], conn)
	if client != nil {
		s.releaseConnectionSlotLocked(client.ip)
	}
	if len(s.clients[streamID]) == 0 {
		delete(s.clients, streamID)
		delete(s.sentStatus, streamID)
	}
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
	s.activeClients = 0
	s.clientCounts = make(map[string]int)
	s.sentStatus = make(map[string]types.StreamStatus)
	s.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}
