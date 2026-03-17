package requests

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type idempotencyEntry struct {
	requestID uuid.UUID
	response  map[string]any
	expiresAt time.Time
}

type InMemoryIdempotencyStore struct {
	mu   sync.RWMutex
	keys map[string]idempotencyEntry
}

func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{keys: make(map[string]idempotencyEntry)}
}

func (s *InMemoryIdempotencyStore) Get(ctx context.Context, key string) (requestID uuid.UUID, response map[string]any, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.keys[key]
	if !ok || time.Now().After(e.expiresAt) {
		return uuid.Nil, nil, false
	}
	return e.requestID, e.response, true
}

func (s *InMemoryIdempotencyStore) Set(ctx context.Context, key string, requestID uuid.UUID, response map[string]any, expiresAt interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var t time.Time
	switch v := expiresAt.(type) {
	case time.Time:
		t = v
	case *time.Time:
		if v != nil {
			t = *v
		} else {
			t = time.Now().Add(24 * time.Hour)
		}
	default:
		t = time.Now().Add(24 * time.Hour)
	}
	s.keys[key] = idempotencyEntry{requestID: requestID, response: response, expiresAt: t}
	return nil
}
