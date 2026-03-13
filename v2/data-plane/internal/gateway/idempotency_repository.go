package gateway

import (
	"context"
	"errors"
	"sync"
	"time"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

var ErrIdempotencyNotFound = errors.New("idempotency record not found")

type InMemoryIdempotencyRepository struct {
	mu    sync.RWMutex
	items map[string]gwdomain.IdempotencyRecord
}

func NewInMemoryIdempotencyRepository() *InMemoryIdempotencyRepository {
	return &InMemoryIdempotencyRepository{
		items: map[string]gwdomain.IdempotencyRecord{},
	}
}

func (r *InMemoryIdempotencyRepository) Get(_ context.Context, toolName, key string) (*gwdomain.IdempotencyRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[idempotencyStoreKey(toolName, key)]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (r *InMemoryIdempotencyRepository) CreateInProgress(_ context.Context, rec gwdomain.IdempotencyRecord) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	storeKey := idempotencyStoreKey(rec.ToolName, rec.IdempotencyKey)
	if _, exists := r.items[storeKey]; exists {
		return false, nil
	}

	now := time.Now().UTC()
	rec.Status = gwdomain.IdempotencyStatusInProgress
	rec.CreatedAt = now
	rec.UpdatedAt = now
	r.items[storeKey] = rec
	return true, nil
}

func (r *InMemoryIdempotencyRepository) MarkCompleted(_ context.Context, toolName, key string, snapshot gwdomain.IdempotencyResponseSnapshot) error {
	return r.update(toolName, key, gwdomain.IdempotencyStatusCompleted, snapshot)
}

func (r *InMemoryIdempotencyRepository) MarkFailed(_ context.Context, toolName, key string, snapshot gwdomain.IdempotencyResponseSnapshot) error {
	return r.update(toolName, key, gwdomain.IdempotencyStatusFailed, snapshot)
}

func (r *InMemoryIdempotencyRepository) update(toolName, key string, status gwdomain.IdempotencyRecordStatus, snapshot gwdomain.IdempotencyResponseSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	storeKey := idempotencyStoreKey(toolName, key)
	item, ok := r.items[storeKey]
	if !ok {
		return ErrIdempotencyNotFound
	}

	item.Status = status
	item.Response = snapshot
	item.UpdatedAt = time.Now().UTC()
	r.items[storeKey] = item
	return nil
}

func idempotencyStoreKey(toolName, key string) string {
	return toolName + "\x00" + key
}
