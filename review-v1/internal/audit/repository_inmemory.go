package audit

import (
	"context"
	"sync"

	"github.com/google/uuid"
	auditdomain "github.com/devpablocristo/nexus/review-v1/internal/audit/usecases/domain"
)

type InMemoryRepository struct {
	mu     sync.RWMutex
	events []auditdomain.RequestEvent
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{events: make([]auditdomain.RequestEvent, 0)}
}

func (r *InMemoryRepository) Append(ctx context.Context, e auditdomain.RequestEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	r.events = append(r.events, e)
	return nil
}

func (r *InMemoryRepository) ListByRequestID(ctx context.Context, requestID uuid.UUID) ([]auditdomain.RequestEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []auditdomain.RequestEvent
	for _, e := range r.events {
		if e.RequestID == requestID {
			out = append(out, e)
		}
	}
	return out, nil
}
