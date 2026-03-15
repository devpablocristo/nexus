package resources

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
)

var ErrNotFound = errors.New("resource not found")
var ErrAlreadyArchived = errors.New("resource already archived")
var ErrNotArchived = errors.New("resource not archived")
var ErrArchived = errors.New("resource archived")

type ListFilters struct {
	Type        string
	Environment string
	Archived    *bool
	Limit       int
}

type Repository interface {
	Create(ctx context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error)
	List(ctx context.Context, filters ListFilters) ([]resourcedomain.ProtectedResource, error)
	GetByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error)
	Update(ctx context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (resourcedomain.ProtectedResource, error)
	Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (resourcedomain.ProtectedResource, error)
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]resourcedomain.ProtectedResource
}

func NewInMemoryRepository(items []resourcedomain.ProtectedResource) *InMemoryRepository {
	store := make(map[uuid.UUID]resourcedomain.ProtectedResource, len(items))
	for _, item := range items {
		if parsed, err := uuid.Parse(item.ID); err == nil {
			store[parsed] = item
		}
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error) {
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

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]resourcedomain.ProtectedResource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]resourcedomain.ProtectedResource, 0, len(r.items))
	for _, item := range r.items {
		if filters.Type != "" && string(item.Type) != filters.Type {
			continue
		}
		if filters.Environment != "" && item.Environment != filters.Environment {
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
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if filters.Limit > 0 && len(items) > filters.Limit {
		items = items[:filters.Limit]
	}
	return items, nil
}

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Update(_ context.Context, item resourcedomain.ProtectedResource) (resourcedomain.ProtectedResource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := uuid.Parse(item.ID)
	if err != nil {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	current, ok := r.items[id]
	if !ok {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	if current.ArchivedAt != nil {
		return resourcedomain.ProtectedResource{}, ErrArchived
	}

	item.CreatedAt = current.CreatedAt
	item.ArchivedAt = current.ArchivedAt
	item.UpdatedAt = time.Now().UTC()
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *InMemoryRepository) Archive(_ context.Context, id uuid.UUID, archivedAt time.Time) (resourcedomain.ProtectedResource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	if item.ArchivedAt != nil {
		return resourcedomain.ProtectedResource{}, ErrAlreadyArchived
	}
	item.ArchivedAt = &archivedAt
	item.UpdatedAt = archivedAt
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) Restore(_ context.Context, id uuid.UUID, restoredAt time.Time) (resourcedomain.ProtectedResource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return resourcedomain.ProtectedResource{}, ErrNotFound
	}
	if item.ArchivedAt == nil {
		return resourcedomain.ProtectedResource{}, ErrNotArchived
	}
	item.ArchivedAt = nil
	item.UpdatedAt = restoredAt
	r.items[id] = item
	return item, nil
}
