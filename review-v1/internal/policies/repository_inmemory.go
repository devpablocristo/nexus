package policies

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	policydomain "github.com/devpablocristo/nexus/review-v1/internal/policies/usecases/domain"
)

type InMemoryRepository struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]policydomain.Policy
	order []uuid.UUID
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{byID: make(map[uuid.UUID]policydomain.Policy)}
}

func (r *InMemoryRepository) Create(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	r.byID[p.ID] = p
	r.order = append(r.order, p.ID)
	return p, nil
}

func (r *InMemoryRepository) GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	return p, nil
}

func (r *InMemoryRepository) List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []policydomain.Policy
	for _, id := range r.order {
		p := r.byID[id]
		if !filters.IncludeArchived && p.ArchivedAt != nil {
			continue
		}
		if filters.EnabledOnly && !p.Enabled {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (r *InMemoryRepository) Update(ctx context.Context, p policydomain.Policy) (policydomain.Policy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[p.ID]; !ok {
		return policydomain.Policy{}, ErrNotFound
	}
	p.UpdatedAt = time.Now().UTC()
	r.byID[p.ID] = p
	return p, nil
}

func (r *InMemoryRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return ErrNotFound
	}
	delete(r.byID, id)
	for i, oid := range r.order {
		if oid == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

func (r *InMemoryRepository) ArchiveByID(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	// Idempotente: si ya está archivado, no hacer nada
	if p.ArchivedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	p.ArchivedAt = &now
	p.UpdatedAt = now
	r.byID[id] = p
	return nil
}

func (r *InMemoryRepository) RestoreByID(ctx context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok {
		return ErrNotFound
	}
	// Idempotente: si no está archivado, no hacer nada
	if p.ArchivedAt == nil {
		return nil
	}
	p.ArchivedAt = nil
	p.UpdatedAt = time.Now().UTC()
	r.byID[id] = p
	return nil
}
