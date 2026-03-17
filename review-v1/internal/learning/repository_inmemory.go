package learning

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
)

type InMemoryRepository struct {
	mu    sync.RWMutex
	byID  map[uuid.UUID]learningdomain.PolicyProposal
	order []uuid.UUID
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{byID: make(map[uuid.UUID]learningdomain.PolicyProposal)}
}

func (r *InMemoryRepository) CreateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	r.byID[p.ID] = p
	r.order = append(r.order, p.ID)
	return p, nil
}

func (r *InMemoryRepository) ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []learningdomain.PolicyProposal
	for _, id := range r.order {
		p := r.byID[id]
		if p.Status != learningdomain.ProposalStatusPending {
			continue
		}
		out = append(out, p)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *InMemoryRepository) GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return learningdomain.PolicyProposal{}, ErrNotFound
	}
	return p, nil
}

func (r *InMemoryRepository) UpdateProposal(ctx context.Context, p learningdomain.PolicyProposal) (learningdomain.PolicyProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[p.ID] = p
	return p, nil
}

var ErrNotFound = errors.New("proposal not found")
