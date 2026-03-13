package policy

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	policydomain "nexus/v2/data-plane/internal/policy/usecases/domain"
)

var ErrNotFound = errors.New("policy not found")

type ListFilters struct {
	ToolName        string
	IncludeArchived bool
}

// InMemoryRepository stores policies in memory for v2.
type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]policydomain.Policy
}

// NewInMemoryRepository builds an in-memory policy repository.
func NewInMemoryRepository(items []policydomain.Policy) *InMemoryRepository {
	store := make(map[uuid.UUID]policydomain.Policy, len(items))
	for _, item := range items {
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		now := time.Now().UTC()
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = item.CreatedAt
		}
		store[item.ID] = item
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	r.items[item.ID] = item
	return item, nil
}

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]policydomain.Policy, 0, len(r.items))
	for _, item := range r.items {
		if filters.ToolName != "" && item.ToolName != filters.ToolName {
			continue
		}
		if item.Archived && !filters.IncludeArchived {
			continue
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

	if _, ok := r.items[item.ID]; !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	item.UpdatedAt = time.Now().UTC()
	r.items[item.ID] = item
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
	now := time.Now().UTC()
	item.Archived = true
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
	item.Archived = false
	item.ArchivedAt = nil
	item.UpdatedAt = time.Now().UTC()
	r.items[id] = item
	return item, nil
}

// ListByToolName returns enabled policies for a tool ordered by priority.
func (r *InMemoryRepository) ListByToolName(_ context.Context, toolName string) ([]policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filtered := make([]policydomain.Policy, 0, len(r.items))
	for _, item := range r.items {
		if !item.Enabled || item.Archived || item.ToolName != toolName {
			continue
		}
		filtered = append(filtered, item)
	}
	sortPolicies(filtered)
	return filtered, nil
}

func sortPolicies(items []policydomain.Policy) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}
