package registry

import (
	"sync"
	"time"

	"github.com/saveenergy/openbyte/pkg/types"
)

type Service struct {
	servers    map[string]*RegisteredServer
	mu         sync.RWMutex
	ttl        time.Duration
	cleanupInt time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
	startOnce  sync.Once
}

type RegisteredServer struct {
	types.ServerInfo
	LastSeen  time.Time `json:"last_seen"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewService(ttl, cleanupInterval time.Duration) *Service {
	if ttl == 0 {
		ttl = 60 * time.Second
	}
	if cleanupInterval == 0 {
		cleanupInterval = 30 * time.Second
	}

	return &Service{
		servers:    make(map[string]*RegisteredServer),
		ttl:        ttl,
		cleanupInt: cleanupInterval,
		stopCh:     make(chan struct{}),
	}
}

func (s *Service) Start() {
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go s.cleanupLoop()
	})
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *Service) Register(info types.ServerInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.servers[info.ID] = &RegisteredServer{
		ServerInfo: info,
		LastSeen:   now,
		ExpiresAt:  now.Add(s.ttl),
	}
}

func (s *Service) Update(id string, info types.ServerInfo) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.servers[id]; !exists {
		return false
	}

	now := time.Now()
	s.servers[id] = &RegisteredServer{
		ServerInfo: info,
		LastSeen:   now,
		ExpiresAt:  now.Add(s.ttl),
	}
	return true
}

func (s *Service) UpdatePatched(id string, patch func(*types.ServerInfo)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.servers[id]
	if !exists {
		return false
	}

	updated := current.ServerInfo
	patch(&updated)
	updated.ID = id
	now := time.Now()
	s.servers[id] = &RegisteredServer{
		ServerInfo: updated,
		LastSeen:   now,
		ExpiresAt:  now.Add(s.ttl),
	}
	return true
}

func (s *Service) Deregister(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.servers[id]; !exists {
		return false
	}

	delete(s.servers, id)
	return true
}

func (s *Service) Get(id string) (*RegisteredServer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	server, exists := s.servers[id]
	if !exists {
		return nil, false
	}

	dup := *server
	return &dup, true
}

func (s *Service) List() []RegisteredServer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RegisteredServer, 0, len(s.servers))
	for _, server := range s.servers {
		result = append(result, *server)
	}
	return result
}

func (s *Service) ListHealthy() []RegisteredServer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	result := make([]RegisteredServer, 0, len(s.servers))
	for _, server := range s.servers {
		if server.ExpiresAt.After(now) && server.Health == "healthy" {
			result = append(result, *server)
		}
	}
	return result
}

func (s *Service) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.servers)
}

func (s *Service) cleanupLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cleanupInt)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *Service) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for id, server := range s.servers {
		if server.ExpiresAt.Before(now) {
			delete(s.servers, id)
		}
	}
}
