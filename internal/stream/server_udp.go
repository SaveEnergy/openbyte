package stream

import (
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/saveenergy/openbyte/internal/logging"
)

const (
	udpCmdDownload = 'D'
	udpCmdUpload   = 'U'
	udpCmdStop     = 'S'
)

// udpClients tracks active UDP client state, safe for concurrent access.
type udpClients struct {
	mu sync.RWMutex
	m  map[string]*udpClientState
}

func (c *udpClients) get(key string) *udpClientState {
	c.mu.RLock()
	client := c.m[key]
	c.mu.RUnlock()
	return client
}

// getOrCreate returns an existing client or creates a new one.
// Returns (nil, false) if the sender limit is reached.
// Returns (client, true) if a new client was created (caller must start sender).
func (c *udpClients) getOrCreate(key string, addr *net.UDPAddr, s *Server) (*udpClientState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.m[key]; existing != nil {
		return existing, false
	}
	if atomic.AddInt64(&s.activeUDPSenders, 1) > s.maxUDPSenders {
		atomic.AddInt64(&s.activeUDPSenders, -1)
		return nil, false
	}
	client := &udpClientState{
		addr:         addr,
		senderActive: 1,
		lastSeenUnix: time.Now().UnixNano(),
	}
	c.m[key] = client
	return client, true
}

func (c *udpClients) cleanup() {
	now := time.Now()
	c.mu.Lock()
	for key, client := range c.m {
		lastSeen := time.Unix(0, atomic.LoadInt64(&client.lastSeenUnix))
		if now.Sub(lastSeen) > 30*time.Second && atomic.LoadInt32(&client.senderActive) == 0 {
			delete(c.m, key)
		}
	}
	c.mu.Unlock()
}

func (c *udpClients) remove(key string) {
	c.mu.Lock()
	delete(c.m, key)
	c.mu.Unlock()
}

func (s *Server) handleUDP() {
	defer s.wg.Done()

	clients := &udpClients{m: make(map[string]*udpClientState)}

	numReaders := max(runtime.GOMAXPROCS(0), 2)
	if numReaders > 4 {
		numReaders = 4
	}

	var readersWg sync.WaitGroup
	for i := 0; i < numReaders; i++ {
		readersWg.Add(1)
		go s.udpReader(clients, &readersWg)
	}

	logging.Info("UDP readers started", logging.Field{Key: "count", Value: numReaders})

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			readersWg.Wait()
			return
		case <-ticker.C:
			clients.cleanup()
		}
	}
}

func (s *Server) udpReader(clients *udpClients, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, s.config.UDPBufferSize)

	for {
		_ = s.udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, addr, err := s.udpConn.ReadFromUDP(buf)
		if err != nil {
			if s.isStopping() {
				return
			}
			if isTimeoutError(err) {
				continue
			}
			logging.Warn("UDP read error", logging.Field{Key: "error", Value: err})
			return
		}

		if n == 0 {
			continue
		}

		clientKey := addr.String()
		client := clients.get(clientKey)

		if client == nil {
			var created bool
			client, created = clients.getOrCreate(clientKey, addr, s)
			if client == nil {
				continue
			}
			if created {
				s.wg.Add(1)
				go s.udpSender(clients, clientKey, client)
			}
		}

		atomic.StoreInt64(&client.lastSeenUnix, time.Now().UnixNano())

		switch buf[0] {
		case udpCmdDownload:
			atomic.StoreInt32(&client.downloading, 1)
		case udpCmdUpload:
			atomic.AddInt64(&client.bytesRecv, int64(n))
		case udpCmdStop:
			atomic.StoreInt32(&client.downloading, 0)
		default:
			if _, err := s.udpConn.WriteToUDP(buf[:n], addr); err != nil {
				logging.Warn("UDP echo error", logging.Field{Key: "error", Value: err})
			}
		}
	}
}

type udpClientState struct {
	addr         *net.UDPAddr
	downloading  int32
	senderActive int32
	bytesRecv    int64
	lastSeenUnix int64
}

func (s *Server) udpSender(clients *udpClients, clientKey string, client *udpClientState) {
	defer s.wg.Done()
	defer atomic.AddInt64(&s.activeUDPSenders, -1)
	defer atomic.StoreInt32(&client.senderActive, 0)
	defer clients.remove(clientKey)
	defer func() {
		if s.udpConn != nil {
			_ = s.udpConn.SetWriteDeadline(time.Time{})
		}
	}()

	packet := make([]byte, s.config.UDPBufferSize)
	n := min(len(packet), len(s.randomData))
	copy(packet, s.randomData[:n])

	lastYield := time.Now()
	for {
		select {
		case <-s.stopCh:
			return
		default:
			lastSeen := time.Unix(0, atomic.LoadInt64(&client.lastSeenUnix))
			if time.Since(lastSeen) > 30*time.Second {
				return
			}
			if atomic.LoadInt32(&client.downloading) == 1 {
				_ = s.udpConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := s.udpConn.WriteToUDP(packet, client.addr); err != nil {
					logging.Warn("UDP send error", logging.Field{Key: "error", Value: err})
					return
				}
				if time.Since(lastYield) > 2*time.Millisecond {
					runtime.Gosched()
					lastYield = time.Now()
				}
				continue
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
