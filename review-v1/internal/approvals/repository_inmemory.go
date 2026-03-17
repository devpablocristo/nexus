package approvals

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	approvaldomain "github.com/devpablocristo/nexus/review-v1/internal/approvals/usecases/domain"
)

type InMemoryRepository struct {
	mu      sync.RWMutex
	byID    map[uuid.UUID]approvaldomain.Approval
	byReqID map[uuid.UUID]uuid.UUID
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		byID:    make(map[uuid.UUID]approvaldomain.Approval),
		byReqID: make(map[uuid.UUID]uuid.UUID),
	}
}

func (r *InMemoryRepository) Create(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.byID[a.ID] = a
	r.byReqID[a.RequestID] = a.ID
	return a, nil
}

func (r *InMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (approvaldomain.Approval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.byID[id]
	if !ok {
		return approvaldomain.Approval{}, ErrNotFound
	}
	return a, nil
}

func (r *InMemoryRepository) GetByRequestID(ctx context.Context, requestID uuid.UUID) (*approvaldomain.Approval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byReqID[requestID]
	if !ok {
		return nil, nil
	}
	a := r.byID[id]
	return &a, nil
}

func (r *InMemoryRepository) ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []approvaldomain.Approval
	for _, a := range r.byID {
		if a.Status != approvaldomain.ApprovalStatusPending {
			continue
		}
		out = append(out, a)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *InMemoryRepository) Update(ctx context.Context, a approvaldomain.Approval) (approvaldomain.Approval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[a.ID] = a
	return a, nil
}

var ErrNotFound = errors.New("approval not found")
