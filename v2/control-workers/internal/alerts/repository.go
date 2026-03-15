package alerts

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	alertdomain "nexus/v2/control-workers/internal/alerts/usecases/domain"
)

var ErrNotFound = errors.New("alert not found")
var ErrArchived = errors.New("alert archived")
var ErrAlreadyArchived = errors.New("alert already archived")
var ErrNotArchived = errors.New("alert not archived")

type ListFilters struct {
	SourceKind string
	Channel    string
	Severity   string
	Status     string
	Archived   *bool
	Limit      int
}

type Repository interface {
	Create(ctx context.Context, item alertdomain.Alert) (alertdomain.Alert, error)
	List(ctx context.Context, filters ListFilters) ([]alertdomain.Alert, error)
	GetByID(ctx context.Context, id uuid.UUID) (alertdomain.Alert, error)
	Update(ctx context.Context, item alertdomain.Alert) (alertdomain.Alert, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (alertdomain.Alert, error)
	Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (alertdomain.Alert, error)
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]alertdomain.Alert
}

func NewInMemoryRepository(items []alertdomain.Alert) *InMemoryRepository {
	store := make(map[uuid.UUID]alertdomain.Alert, len(items))
	for _, item := range items {
		if parsed, err := uuid.Parse(item.ID); err == nil {
			store[parsed] = item
		}
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item alertdomain.Alert) (alertdomain.Alert, error) {
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

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]alertdomain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]alertdomain.Alert, 0, len(r.items))
	for _, item := range r.items {
		if filters.SourceKind != "" && string(item.SourceKind) != filters.SourceKind {
			continue
		}
		if filters.Channel != "" && string(item.Channel) != filters.Channel {
			continue
		}
		if filters.Severity != "" && string(item.Severity) != filters.Severity {
			continue
		}
		if filters.Status != "" && string(item.Status) != filters.Status {
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

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (alertdomain.Alert, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return alertdomain.Alert{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Update(_ context.Context, item alertdomain.Alert) (alertdomain.Alert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := uuid.Parse(item.ID)
	if err != nil {
		return alertdomain.Alert{}, ErrNotFound
	}
	current, ok := r.items[id]
	if !ok {
		return alertdomain.Alert{}, ErrNotFound
	}
	if current.ArchivedAt != nil {
		return alertdomain.Alert{}, ErrArchived
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

func (r *InMemoryRepository) Archive(_ context.Context, id uuid.UUID, archivedAt time.Time) (alertdomain.Alert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return alertdomain.Alert{}, ErrNotFound
	}
	if item.ArchivedAt != nil {
		return alertdomain.Alert{}, ErrAlreadyArchived
	}
	item.ArchivedAt = &archivedAt
	item.UpdatedAt = archivedAt
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) Restore(_ context.Context, id uuid.UUID, restoredAt time.Time) (alertdomain.Alert, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return alertdomain.Alert{}, ErrNotFound
	}
	if item.ArchivedAt == nil {
		return alertdomain.Alert{}, ErrNotArchived
	}
	item.ArchivedAt = nil
	item.UpdatedAt = restoredAt
	r.items[id] = item
	return item, nil
}
