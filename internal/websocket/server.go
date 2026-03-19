package websocket

import (
	"bytes"
	"net/http"
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

const internalWSClientIPHeader = "X-OpenByte-Client-IP"

type Server struct {
	upgrader        websocket.Upgrader
	clients         map[string]map[*websocket.Conn]*clientConn
	clientCounts    map[string]int
	sentStatus      map[string]types.StreamStatus
	allowedOrigins  []string
	pingInterval    time.Duration
	maxClients      int
	maxClientsPerIP int
	activeClients   int
	stopCh          chan struct{}
	stopOnce        sync.Once
	wg              sync.WaitGroup
	jsonBufPool     sync.Pool
	mu              sync.RWMutex
}

type clientConn struct {
	conn *websocket.Conn
	ip   string
	mu   sync.Mutex
}

func NewServer() *Server {
	server := &Server{
		clients:      make(map[string]map[*websocket.Conn]*clientConn),
		clientCounts: make(map[string]int),
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

func (s *Server) SetConnectionLimits(maxClients, maxClientsPerIP int) {
	if maxClients < 0 {
		maxClients = 0
	}
	if maxClientsPerIP < 0 {
		maxClientsPerIP = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxClients = maxClients
	s.maxClientsPerIP = maxClientsPerIP
}

func (s *Server) HandleStream(w http.ResponseWriter, r *http.Request, streamID string) {
	clientIP := websocketClientIP(r)
	if !s.reserveConnectionSlot(clientIP) {
		http.Error(w, "too many websocket clients", http.StatusServiceUnavailable)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.releaseConnectionSlot(clientIP)
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
	client := &clientConn{conn: conn, ip: clientIP}
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
