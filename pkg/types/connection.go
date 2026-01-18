package types

import (
	"net"
	"sync"
	"time"
)

type ConnectionState struct {
	ID             int
	LocalAddr      net.Addr
	RemoteAddr     net.Addr
	BytesSent      int64
	BytesRecv      int64
	PacketsSent    int64
	PacketsRecv    int64
	LatencySamples []time.Duration
	LastError      error
	Status         string
	Mu             sync.RWMutex
}

type ConnectionPool interface {
	CreateConnections(count int) ([]net.Conn, error)
	CloseConnections() error
	GetConnectionStates() []ConnectionState
	Wait() error
}
