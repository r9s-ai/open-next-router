package onrserver

import (
	"sync"

	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
)

type state struct {
	mu          sync.RWMutex
	keys        *keystore.Store
	modelRouter *models.Router
	startedAt   int64
}

func (s *state) Keys() *keystore.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.keys
}

func (s *state) SetKeys(keys *keystore.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = keys
}

func (s *state) ModelRouter() *models.Router {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.modelRouter
}

func (s *state) SetModelRouter(r *models.Router) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modelRouter = r
}

func (s *state) StartedAtUnix() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

func (s *state) SetStartedAtUnix(ts int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startedAt = ts
}
