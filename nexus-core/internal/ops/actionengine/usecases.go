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

type Service interface {
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

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) UpsertCatalog(ctx context.Context, item actiondomain.CatalogItem) error {
	return s.repo.UpsertCatalog(ctx, item)
}

func (s *service) GetCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error) {
	return s.repo.GetCatalog(ctx, actionType)
}

func (s *service) CreateProposal(ctx context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error) {
	return s.repo.CreateProposal(ctx, in)
}

func (s *service) GetProposalByID(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Proposal, error) {
	return s.repo.GetProposalByID(ctx, orgID, proposalID)
}

func (s *service) GetProposalByIdempotencyKey(ctx context.Context, orgID uuid.UUID, idempotencyKey string) (actiondomain.Proposal, error) {
	return s.repo.GetProposalByIdempotencyKey(ctx, orgID, idempotencyKey)
}

func (s *service) UpdateProposalStatus(ctx context.Context, orgID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error) {
	return s.repo.UpdateProposalStatus(ctx, orgID, proposalID, status)
}

func (s *service) CreateExecution(ctx context.Context, in actiondomain.Execution) (actiondomain.Execution, error) {
	return s.repo.CreateExecution(ctx, in)
}

func (s *service) CreateApproval(ctx context.Context, in actiondomain.Approval) (actiondomain.Approval, error) {
	return s.repo.CreateApproval(ctx, in)
}

func (s *service) GetLatestApproval(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Approval, error) {
	return s.repo.GetLatestApproval(ctx, orgID, proposalID)
}
