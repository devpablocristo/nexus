package incidents

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	incidentdomain "nexus/v2/control-workers/internal/incidents/usecases/domain"
)

var ErrNotFound = errors.New("incident not found")
var ErrArchived = errors.New("incident archived")
var ErrAlreadyArchived = errors.New("incident already archived")
var ErrNotArchived = errors.New("incident not archived")

type ListFilters struct {
	SourceKind string
	ResourceID string
	Trigger    string
	Severity   string
	Status     string
	Archived   *bool
	Limit      int
}

type Repository interface {
	Create(ctx context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error)
	List(ctx context.Context, filters ListFilters) ([]incidentdomain.Incident, error)
	GetByID(ctx context.Context, id uuid.UUID) (incidentdomain.Incident, error)
	Update(ctx context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (incidentdomain.Incident, error)
	Restore(ctx context.Context, id uuid.UUID, restoredAt time.Time) (incidentdomain.Incident, error)
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]incidentdomain.Incident
}

func NewInMemoryRepository(items []incidentdomain.Incident) *InMemoryRepository {
	store := make(map[uuid.UUID]incidentdomain.Incident, len(items))
	for _, item := range items {
		if parsed, err := uuid.Parse(item.ID); err == nil {
			store[parsed] = item
		}
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error) {
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

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]incidentdomain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]incidentdomain.Incident, 0, len(r.items))
	for _, item := range r.items {
		if filters.SourceKind != "" && string(item.SourceKind) != filters.SourceKind {
			continue
		}
		if filters.ResourceID != "" && item.ResourceID != filters.ResourceID {
			continue
		}
		if filters.Trigger != "" && string(item.Trigger) != filters.Trigger {
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

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (incidentdomain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return incidentdomain.Incident{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Update(_ context.Context, item incidentdomain.Incident) (incidentdomain.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := uuid.Parse(item.ID)
	if err != nil {
		return incidentdomain.Incident{}, ErrNotFound
	}
	current, ok := r.items[id]
	if !ok {
		return incidentdomain.Incident{}, ErrNotFound
	}
	if current.ArchivedAt != nil {
		return incidentdomain.Incident{}, ErrArchived
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

func (r *InMemoryRepository) Archive(_ context.Context, id uuid.UUID, archivedAt time.Time) (incidentdomain.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return incidentdomain.Incident{}, ErrNotFound
	}
	if item.ArchivedAt != nil {
		return incidentdomain.Incident{}, ErrAlreadyArchived
	}
	item.ArchivedAt = &archivedAt
	item.UpdatedAt = archivedAt
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) Restore(_ context.Context, id uuid.UUID, restoredAt time.Time) (incidentdomain.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return incidentdomain.Incident{}, ErrNotFound
	}
	if item.ArchivedAt == nil {
		return incidentdomain.Incident{}, ErrNotArchived
	}
	item.ArchivedAt = nil
	item.UpdatedAt = restoredAt
	r.items[id] = item
	return item, nil
}
