package gateway

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

var ErrIntentNotFound = errors.New("intent not found")

type InMemoryIntentRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]gwdomain.ExecutionIntent
}

func NewInMemoryIntentRepository() *InMemoryIntentRepository {
	return &InMemoryIntentRepository{
		items: make(map[uuid.UUID]gwdomain.ExecutionIntent),
	}
}

func (r *InMemoryIntentRepository) Create(_ context.Context, intent gwdomain.ExecutionIntent) (gwdomain.ExecutionIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if intent.ID == uuid.Nil {
		intent.ID = uuid.New()
	}
	if intent.CreatedAt.IsZero() {
		intent.CreatedAt = now
	}
	intent.UpdatedAt = now
	r.items[intent.ID] = intent
	return intent, nil
}

func (r *InMemoryIntentRepository) LinkApproval(_ context.Context, intentID, approvalID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[intentID]
	if !ok {
		return ErrIntentNotFound
	}
	item.ApprovalID = &approvalID
	item.UpdatedAt = time.Now().UTC()
	r.items[intentID] = item
	return nil
}

func (r *InMemoryIntentRepository) GetByID(_ context.Context, intentID uuid.UUID) (gwdomain.ExecutionIntent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[intentID]
	if !ok {
		return gwdomain.ExecutionIntent{}, ErrIntentNotFound
	}
	return item, nil
}

func (r *InMemoryIntentRepository) ListRecent(_ context.Context, limit int) ([]gwdomain.ExecutionIntent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]gwdomain.ExecutionIntent, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *InMemoryIntentRepository) MarkApproved(_ context.Context, intentID uuid.UUID) error {
	return r.updateStatus(intentID, gwdomain.IntentStatusApproved)
}

func (r *InMemoryIntentRepository) MarkRejected(_ context.Context, intentID uuid.UUID) error {
	return r.updateStatus(intentID, gwdomain.IntentStatusRejected)
}

func (r *InMemoryIntentRepository) MarkExecuted(_ context.Context, intentID uuid.UUID) error {
	return r.updateStatus(intentID, gwdomain.IntentStatusExecuted)
}

func (r *InMemoryIntentRepository) updateStatus(intentID uuid.UUID, status gwdomain.IntentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[intentID]
	if !ok {
		return ErrIntentNotFound
	}
	item.Status = status
	now := time.Now().UTC()
	item.UpdatedAt = now
	if status == gwdomain.IntentStatusExecuted {
		item.ExecutedAt = &now
	}
	r.items[intentID] = item
	return nil
}
