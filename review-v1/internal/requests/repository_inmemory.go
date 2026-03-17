package requests

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	requestdomain "github.com/devpablocristo/nexus/review-v1/internal/requests/usecases/domain"
)

type InMemoryRepository struct {
	mu   sync.RWMutex
	byID map[uuid.UUID]requestdomain.Request
	byKey map[string]uuid.UUID
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		byID:  make(map[uuid.UUID]requestdomain.Request),
		byKey: make(map[string]uuid.UUID),
	}
}

func (r *InMemoryRepository) Create(ctx context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	if req.IdempotencyKey != nil {
		r.byKey[*req.IdempotencyKey] = req.ID
	}
	r.byID[req.ID] = req
	return req, nil
}

func (r *InMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	req, ok := r.byID[id]
	if !ok {
		return requestdomain.Request{}, ErrNotFound
	}
	return req, nil
}

func (r *InMemoryRepository) GetByIdempotencyKey(ctx context.Context, key string) (*requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byKey[key]
	if !ok {
		return nil, nil
	}
	req, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	return &req, nil
}

func (r *InMemoryRepository) List(ctx context.Context, status, actionType string, limit int) ([]requestdomain.Request, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []requestdomain.Request
	for _, req := range r.byID {
		if status != "" && string(req.Status) != status {
			continue
		}
		if actionType != "" && req.ActionType != actionType {
			continue
		}
		out = append(out, req)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *InMemoryRepository) Update(ctx context.Context, req requestdomain.Request) (requestdomain.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[req.ID]; !ok {
		return requestdomain.Request{}, ErrNotFound
	}
	r.byID[req.ID] = req
	return req, nil
}

var ErrNotFound = errors.New("request not found")
