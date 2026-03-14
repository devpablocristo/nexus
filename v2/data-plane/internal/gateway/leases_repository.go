package gateway

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
)

var ErrLeaseNotFound = errors.New("lease not found")
var ErrLeaseNotActive = errors.New("lease not active")
var ErrLeaseIntentMismatch = errors.New("lease intent mismatch")
var ErrLeaseExpired = errors.New("lease expired")

type InMemoryLeaseRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]gwdomain.ExecutionLease
}

func NewInMemoryLeaseRepository() *InMemoryLeaseRepository {
	return &InMemoryLeaseRepository{
		items: make(map[uuid.UUID]gwdomain.ExecutionLease),
	}
}

func (r *InMemoryLeaseRepository) Create(_ context.Context, lease gwdomain.ExecutionLease) (gwdomain.ExecutionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if lease.ID == uuid.Nil {
		lease.ID = uuid.New()
	}
	if lease.Status == "" {
		lease.Status = gwdomain.ExecutionLeaseStatusActive
	}
	if lease.CredentialMode == "" {
		lease.CredentialMode = "none"
	}
	if lease.CredentialHints == nil {
		lease.CredentialHints = map[string]any{}
	}
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = now
	}
	r.items[lease.ID] = lease
	return lease, nil
}

func (r *InMemoryLeaseRepository) GetByID(_ context.Context, leaseID uuid.UUID) (gwdomain.ExecutionLease, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[leaseID]
	if !ok {
		return gwdomain.ExecutionLease{}, ErrLeaseNotFound
	}
	return item, nil
}

func (r *InMemoryLeaseRepository) Consume(_ context.Context, leaseID, intentID uuid.UUID) (gwdomain.ExecutionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[leaseID]
	if !ok {
		return gwdomain.ExecutionLease{}, ErrLeaseNotFound
	}
	if item.IntentID != intentID {
		return gwdomain.ExecutionLease{}, ErrLeaseIntentMismatch
	}
	if item.Status == gwdomain.ExecutionLeaseStatusExpired {
		return item, ErrLeaseExpired
	}
	if item.Status != gwdomain.ExecutionLeaseStatusActive {
		return item, ErrLeaseNotActive
	}

	now := time.Now().UTC()
	if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
		item.Status = gwdomain.ExecutionLeaseStatusExpired
		r.items[leaseID] = item
		return item, ErrLeaseExpired
	}

	item.Status = gwdomain.ExecutionLeaseStatusUsed
	item.UsedAt = &now
	r.items[leaseID] = item
	return item, nil
}
