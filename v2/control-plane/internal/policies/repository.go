package policies

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
)

var ErrNotFound = errors.New("policy not found")
var ErrArchived = errors.New("policy archived")
var ErrAlreadyArchived = errors.New("policy already archived")
var ErrNotArchived = errors.New("policy not archived")

type ListFilters struct {
	ActionType   string
	ResourceType string
	Archived     *bool
}

type Repository interface {
	Create(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error)
	List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	Save(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]policydomain.Policy
}

func NewInMemoryRepository(items []policydomain.Policy) *InMemoryRepository {
	store := make(map[uuid.UUID]policydomain.Policy, len(items))
	for _, item := range items {
		if parsed, err := uuid.Parse(item.ID); err == nil {
			store[parsed] = item
		}
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	id := uuid.New()
	item.ID = id.String()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]policydomain.Policy, 0, len(r.items))
	for _, item := range r.items {
		if filters.ActionType != "" && item.ActionType != filters.ActionType && item.ActionType != "*" {
			continue
		}
		if filters.ResourceType != "" && item.ResourceType != filters.ResourceType && item.ResourceType != "*" {
			continue
		}
		if filters.Archived != nil {
			archived := item.ArchivedAt != nil
			if archived != *filters.Archived {
				continue
			}
		}
		items = append(items, item)
	}
	sortPolicies(items)
	return items, nil
}

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Save(_ context.Context, item policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := uuid.Parse(item.ID)
	if err != nil {
		return policydomain.Policy{}, ErrNotFound
	}
	current, ok := r.items[id]
	if !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	if current.ArchivedAt != nil {
		return policydomain.Policy{}, ErrArchived
	}

	item.CreatedAt = current.CreatedAt
	item.ArchivedAt = current.ArchivedAt
	item.UpdatedAt = time.Now().UTC()
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) DeleteByID(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *InMemoryRepository) ArchiveByID(_ context.Context, id uuid.UUID) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	if item.ArchivedAt != nil {
		return policydomain.Policy{}, ErrAlreadyArchived
	}
	now := time.Now().UTC()
	item.ArchivedAt = &now
	item.UpdatedAt = now
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) RestoreByID(_ context.Context, id uuid.UUID) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	if item.ArchivedAt == nil {
		return policydomain.Policy{}, ErrNotArchived
	}
	item.ArchivedAt = nil
	item.UpdatedAt = time.Now().UTC()
	r.items[id] = item
	return item, nil
}

func sortPolicies(items []policydomain.Policy) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}
