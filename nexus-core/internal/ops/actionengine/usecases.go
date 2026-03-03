package actionengine

import (
	"context"

	"github.com/google/uuid"
	actiondomain "nexus-core/internal/ops/actionengine/usecases/domain"
)

type RepositoryPort interface {
	UpsertCatalog(ctx context.Context, item actiondomain.CatalogItem) error
	GetCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error)
	CreateProposal(ctx context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error)
	GetProposalByID(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Proposal, error)
	GetProposalByIdempotencyKey(ctx context.Context, orgID uuid.UUID, idempotencyKey string) (actiondomain.Proposal, error)
	UpdateProposalStatus(ctx context.Context, orgID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error)
	CreateExecution(ctx context.Context, in actiondomain.Execution) (actiondomain.Execution, error)
	CreateApproval(ctx context.Context, in actiondomain.Approval) (actiondomain.Approval, error)
	GetLatestApproval(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Approval, error)
}

type Usecases struct {
	repo RepositoryPort
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) UpsertCatalog(ctx context.Context, item actiondomain.CatalogItem) error {
	return u.repo.UpsertCatalog(ctx, item)
}

func (u *Usecases) GetCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error) {
	return u.repo.GetCatalog(ctx, actionType)
}

func (u *Usecases) CreateProposal(ctx context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error) {
	return u.repo.CreateProposal(ctx, in)
}

func (u *Usecases) GetProposalByID(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Proposal, error) {
	return u.repo.GetProposalByID(ctx, orgID, proposalID)
}

func (u *Usecases) GetProposalByIdempotencyKey(ctx context.Context, orgID uuid.UUID, idempotencyKey string) (actiondomain.Proposal, error) {
	return u.repo.GetProposalByIdempotencyKey(ctx, orgID, idempotencyKey)
}

func (u *Usecases) UpdateProposalStatus(ctx context.Context, orgID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error) {
	return u.repo.UpdateProposalStatus(ctx, orgID, proposalID, status)
}

func (u *Usecases) CreateExecution(ctx context.Context, in actiondomain.Execution) (actiondomain.Execution, error) {
	return u.repo.CreateExecution(ctx, in)
}

func (u *Usecases) CreateApproval(ctx context.Context, in actiondomain.Approval) (actiondomain.Approval, error) {
	return u.repo.CreateApproval(ctx, in)
}

func (u *Usecases) GetLatestApproval(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Approval, error) {
	return u.repo.GetLatestApproval(ctx, orgID, proposalID)
}
