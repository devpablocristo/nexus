package action

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

var ErrNotFound = errors.New("action not found")
var ErrApprovalNotPending = errors.New("approval not pending")
var ErrLeaseAlreadyIssued = errors.New("lease already issued")
var ErrLeaseNotFound = errors.New("lease not found")
var ErrLeaseNotActive = errors.New("lease not active")
var ErrLeaseExpired = errors.New("lease expired")
var ErrLeaseMismatch = errors.New("lease mismatch")
var ErrActionAlreadyExecuted = errors.New("action already executed")

type ListFilters struct {
	ActionType string
	Status     string
	Limit      int
}

type HistoryFilters struct {
	ResourceID string
	ActorID    string
	Since      time.Time
	Before     time.Time
	Limit      int
}

// Repository stores actions.
type Repository interface {
	Create(ctx context.Context, item actiondomain.Action) (actiondomain.Action, error)
	List(ctx context.Context, filters ListFilters) ([]actiondomain.Action, error)
	ListHistory(ctx context.Context, filters HistoryFilters) ([]actiondomain.Action, error)
	ListDistinctResourceIDs(ctx context.Context, since time.Time) ([]string, error)
	ListDistinctActorIDs(ctx context.Context, since time.Time) ([]string, error)
	GetByID(ctx context.Context, id uuid.UUID) (actiondomain.Action, error)
	Decide(ctx context.Context, id uuid.UUID, status actiondomain.ApprovalStatus, decidedBy actiondomain.ActorRef, comment string, decidedAt time.Time) (actiondomain.Action, error)
	IssueLease(ctx context.Context, id uuid.UUID, lease actiondomain.ExecutionLease) (actiondomain.Action, error)
	ConsumeLeaseAndMarkExecuted(ctx context.Context, id, leaseID uuid.UUID, execution actiondomain.ExecutionResult) (actiondomain.Action, error)
}

// InMemoryRepository stores actions in memory.
type InMemoryRepository struct {
	mu    sync.RWMutex
	items map[uuid.UUID]actiondomain.Action
}

// NewInMemoryRepository builds an in-memory action repository.
func NewInMemoryRepository(items []actiondomain.Action) *InMemoryRepository {
	store := make(map[uuid.UUID]actiondomain.Action, len(items))
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

func (r *InMemoryRepository) Create(_ context.Context, item actiondomain.Action) (actiondomain.Action, error) {
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

func (r *InMemoryRepository) List(_ context.Context, filters ListFilters) ([]actiondomain.Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]actiondomain.Action, 0, len(r.items))
	for _, item := range r.items {
		if filters.ActionType != "" && string(item.Type) != filters.ActionType {
			continue
		}
		if filters.Status != "" && string(item.Status) != filters.Status {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if filters.Limit > 0 && len(items) > filters.Limit {
		items = items[:filters.Limit]
	}
	return items, nil
}

func (r *InMemoryRepository) ListHistory(_ context.Context, filters HistoryFilters) ([]actiondomain.Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]actiondomain.Action, 0, len(r.items))
	for _, item := range r.items {
		if filters.ResourceID != "" && item.ResourceID != filters.ResourceID {
			continue
		}
		if filters.ActorID != "" && item.ProposedBy.ID != filters.ActorID {
			continue
		}
		if !filters.Since.IsZero() && item.CreatedAt.Before(filters.Since) {
			continue
		}
		if !filters.Before.IsZero() && !item.CreatedAt.Before(filters.Before) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if filters.Limit > 0 && len(items) > filters.Limit {
		items = items[:filters.Limit]
	}
	return items, nil
}

func (r *InMemoryRepository) ListDistinctResourceIDs(_ context.Context, since time.Time) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]struct{}, len(r.items))
	for _, item := range r.items {
		if !since.IsZero() && item.CreatedAt.Before(since) {
			continue
		}
		if item.ResourceID == "" {
			continue
		}
		seen[item.ResourceID] = struct{}{}
	}
	return distinctKeys(seen), nil
}

func (r *InMemoryRepository) ListDistinctActorIDs(_ context.Context, since time.Time) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]struct{}, len(r.items))
	for _, item := range r.items {
		if !since.IsZero() && item.CreatedAt.Before(since) {
			continue
		}
		if item.ProposedBy.ID == "" {
			continue
		}
		seen[item.ProposedBy.ID] = struct{}{}
	}
	return distinctKeys(seen), nil
}

func (r *InMemoryRepository) GetByID(_ context.Context, id uuid.UUID) (actiondomain.Action, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, ok := r.items[id]
	if !ok {
		return actiondomain.Action{}, ErrNotFound
	}
	return item, nil
}

func (r *InMemoryRepository) Decide(_ context.Context, id uuid.UUID, status actiondomain.ApprovalStatus, decidedBy actiondomain.ActorRef, comment string, decidedAt time.Time) (actiondomain.Action, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return actiondomain.Action{}, ErrNotFound
	}
	if item.Approval == nil || item.Approval.Status != actiondomain.ApprovalStatusPending {
		return actiondomain.Action{}, ErrApprovalNotPending
	}

	item.Approval.Status = status
	item.Approval.DecidedBy = &decidedBy
	item.Approval.Comment = comment
	item.Approval.DecidedAt = &decidedAt
	item.Approval.UpdatedAt = decidedAt
	item.UpdatedAt = decidedAt

	switch status {
	case actiondomain.ApprovalStatusApproved:
		item.Approval.GrantedCount = item.Approval.RequiredCount
		item.Status = actiondomain.ActionStatusApproved
		item.Decision = actiondomain.DecisionAllow
	case actiondomain.ApprovalStatusRejected:
		item.Status = actiondomain.ActionStatusRejected
		item.Decision = actiondomain.DecisionDeny
	}

	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) IssueLease(_ context.Context, id uuid.UUID, lease actiondomain.ExecutionLease) (actiondomain.Action, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return actiondomain.Action{}, ErrNotFound
	}
	if item.Lease != nil {
		switch item.Lease.Status {
		case actiondomain.LeaseStatusActive, actiondomain.LeaseStatusUsed:
			return actiondomain.Action{}, ErrLeaseAlreadyIssued
		}
	}

	item.Lease = &lease
	item.Status = actiondomain.ActionStatusLeased
	item.Decision = actiondomain.DecisionAllow
	item.UpdatedAt = lease.CreatedAt
	r.items[id] = item
	return item, nil
}

func (r *InMemoryRepository) ConsumeLeaseAndMarkExecuted(_ context.Context, id, leaseID uuid.UUID, execution actiondomain.ExecutionResult) (actiondomain.Action, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[id]
	if !ok {
		return actiondomain.Action{}, ErrNotFound
	}
	if item.Execution != nil {
		return actiondomain.Action{}, ErrActionAlreadyExecuted
	}
	if item.Lease == nil {
		return actiondomain.Action{}, ErrLeaseNotFound
	}
	if item.Lease.ID != leaseID {
		return actiondomain.Action{}, ErrLeaseMismatch
	}
	if item.Lease.Status != actiondomain.LeaseStatusActive {
		return actiondomain.Action{}, ErrLeaseNotActive
	}
	if !item.Lease.ExpiresAt.IsZero() && execution.ExecutedAt.After(item.Lease.ExpiresAt) {
		item.Lease.Status = actiondomain.LeaseStatusExpired
		item.Status = actiondomain.ActionStatusApproved
		item.UpdatedAt = execution.ExecutedAt
		r.items[id] = item
		return actiondomain.Action{}, ErrLeaseExpired
	}

	item.Lease.Status = actiondomain.LeaseStatusUsed
	item.Lease.UsedAt = &execution.ExecutedAt
	item.Execution = &execution
	item.Status = actiondomain.ActionStatusExecuted
	item.Decision = actiondomain.DecisionAllow
	item.UpdatedAt = execution.ExecutedAt
	r.items[id] = item
	return item, nil
}

func distinctKeys(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}
