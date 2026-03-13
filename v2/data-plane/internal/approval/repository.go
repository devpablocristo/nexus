package approval

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	domain "nexus/v2/data-plane/internal/approval/usecases/domain"
)

var (
	ErrNotFound       = errors.New("approval not found")
	ErrAlreadyDecided = errors.New("approval already decided")
)

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]domain.PendingApproval
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		items: make(map[uuid.UUID]domain.PendingApproval),
	}
}

func (r *InMemoryRepository) Create(_ context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	item := domain.PendingApproval{
		ID:        uuid.New(),
		IntentID:  req.IntentID,
		RequestID: req.RequestID,
		ToolName:  req.ToolName,
		Reason:    req.Reason,
		Status:    domain.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now.Add(time.Duration(req.TTLSeconds) * time.Second),
	}
	r.items[item.ID] = item
	return item, nil
}

func (r *InMemoryRepository) ListPending(_ context.Context, limit int) ([]domain.PendingApproval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]domain.PendingApproval, 0, len(r.items))
	for _, item := range r.items {
		if item.Status != domain.StatusPending {
			continue
		}
		items = append(items, item)
	}
	sortApprovals(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (domain.PendingApproval, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return domain.PendingApproval{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Decide(_ context.Context, id uuid.UUID, status domain.Status, decidedBy string) (domain.PendingApproval, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return domain.PendingApproval{}, ErrNotFound
	}
	if item.Status != domain.StatusPending {
		return domain.PendingApproval{}, ErrAlreadyDecided
	}

	now := time.Now().UTC()
	item.Status = status
	item.DecidedAt = &now
	if decidedBy != "" {
		item.DecidedBy = &decidedBy
	}
	item.UpdatedAt = now
	r.items[id] = item
	return item, nil
}

func sortApprovals(items []domain.PendingApproval) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].CreatedAt.After(items[i].CreatedAt) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
