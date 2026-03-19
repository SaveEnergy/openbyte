package websocket

import (
	"net/http"

	"github.com/saveenergy/openbyte/pkg/types"
)

func (s *Server) reserveConnectionSlot(clientIP string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maxClients > 0 && s.activeClients >= s.maxClients {
		return false
	}
	if clientIP != "" && s.maxClientsPerIP > 0 && s.clientCounts[clientIP] >= s.maxClientsPerIP {
		return false
	}
	s.activeClients++
	if clientIP != "" {
		s.clientCounts[clientIP]++
	}
	return true
}

func (s *Server) releaseConnectionSlot(clientIP string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseConnectionSlotLocked(clientIP)
}

func (s *Server) releaseConnectionSlotLocked(clientIP string) {
	if s.activeClients > 0 {
		s.activeClients--
	}
	if clientIP == "" {
		return
	}
	count := s.clientCounts[clientIP]
	if count <= 1 {
		delete(s.clientCounts, clientIP)
		return
	}
	s.clientCounts[clientIP] = count - 1
}

func websocketClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if clientIP := r.Header.Get(internalWSClientIPHeader); clientIP != "" {
		return clientIP
	}
	return types.StripHostPort(r.RemoteAddr)
}
