package audit

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	auditdomain "nexus/v2/control-plane/internal/audit/usecases/domain"
)

var ErrNotFound = errors.New("audit record not found")

type ListFilters struct {
	ActionID   string
	IncidentID string
	AlertID    string
	ResourceID string
	ActorID    string
	EventType  string
	From       time.Time
	To         time.Time
	Limit      int
}

type Repository interface {
	Create(ctx context.Context, item auditdomain.AuditRecord) (auditdomain.AuditRecord, error)
	List(ctx context.Context, filters ListFilters) ([]auditdomain.AuditRecord, error)
	GetByID(ctx context.Context, id uuid.UUID) (auditdomain.AuditRecord, error)
}

type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]auditdomain.AuditRecord
}

func NewInMemoryRepository(items []auditdomain.AuditRecord) *InMemoryRepository {
	store := make(map[uuid.UUID]auditdomain.AuditRecord, len(items))
	for _, item := range items {
		if parsed, err := uuid.Parse(item.ID); err == nil {
			store[parsed] = item
		}
	}
	return &InMemoryRepository{items: store}
}

func (r *InMemoryRepository) Create(_ context.Context, item auditdomain.AuditRecord) (auditdomain.AuditRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := nowUTC()
	id := uuid.New()
	item.ID = id.String()
	if item.OccurredAt.IsZero() {
		item.OccurredAt = now
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]auditdomain.AuditRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]auditdomain.AuditRecord, 0, len(r.items))
	for _, item := range r.items {
		if filters.ActionID != "" && item.ActionID != filters.ActionID {
			continue
		}
		if filters.IncidentID != "" && item.IncidentID != filters.IncidentID {
			continue
		}
		if filters.AlertID != "" && item.AlertID != filters.AlertID {
			continue
		}
		if filters.ResourceID != "" && item.ResourceID != filters.ResourceID {
			continue
		}
		if filters.ActorID != "" {
			if item.Actor == nil || item.Actor.ID != filters.ActorID {
				continue
			}
		}
		if filters.EventType != "" && item.EventType != filters.EventType {
			continue
		}
		if !filters.From.IsZero() && item.OccurredAt.Before(filters.From) {
			continue
		}
		if !filters.To.IsZero() && item.OccurredAt.After(filters.To) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OccurredAt.Equal(items[j].OccurredAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].OccurredAt.After(items[j].OccurredAt)
	})
	if filters.Limit > 0 && len(items) > filters.Limit {
		items = items[:filters.Limit]
	}
	return items, nil
}

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (auditdomain.AuditRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return auditdomain.AuditRecord{}, ErrNotFound
	}
	return item, nil
}

func cloneData(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func trimOrEmpty(value string) string {
	return strings.TrimSpace(value)
}
