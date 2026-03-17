package learning

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	learningdomain "github.com/devpablocristo/nexus/review-v1/internal/learning/usecases/domain"
)

type Usecases struct {
	repo           Repository
	policyCreator  PolicyCreator
}

func NewUsecases(repo Repository, policyCreator PolicyCreator) *Usecases {
	return &Usecases{repo: repo, policyCreator: policyCreator}
}

func (u *Usecases) ListPendingProposals(ctx context.Context, limit int) ([]learningdomain.PolicyProposal, error) {
	return u.repo.ListPendingProposals(ctx, limit)
}

func (u *Usecases) GetProposalByID(ctx context.Context, id uuid.UUID) (learningdomain.PolicyProposal, error) {
	return u.repo.GetProposalByID(ctx, id)
}

func (u *Usecases) AcceptProposal(ctx context.Context, id uuid.UUID, decidedBy string) (policyID *uuid.UUID, err error) {
	p, err := u.repo.GetProposalByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status != learningdomain.ProposalStatusPending {
		return nil, ErrNotPending
	}
	createdID, err := u.policyCreator.CreateFromProposal(ctx, p)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	p.Status = learningdomain.ProposalStatusAccepted
	p.DecidedBy = &decidedBy
	p.DecidedAt = &now
	p.PolicyID = &createdID
	_, _ = u.repo.UpdateProposal(ctx, p)
	return &createdID, nil
}

func (u *Usecases) DismissProposal(ctx context.Context, id uuid.UUID, decidedBy string) error {
	p, err := u.repo.GetProposalByID(ctx, id)
	if err != nil {
		return err
	}
	if p.Status != learningdomain.ProposalStatusPending {
		return ErrNotPending
	}
	now := time.Now().UTC()
	p.Status = learningdomain.ProposalStatusDismissed
	p.DecidedBy = &decidedBy
	p.DecidedAt = &now
	_, err = u.repo.UpdateProposal(ctx, p)
	return err
}

var ErrNotPending = errors.New("proposal is not pending")
