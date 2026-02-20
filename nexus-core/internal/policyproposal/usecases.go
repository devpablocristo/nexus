package policyproposal

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	eventdomain "nexus-core/internal/events/usecases/domain"
	proposaldomain "nexus-core/internal/policyproposal/usecases/domain"
	"nexus-core/pkg/types"
)

type RepositoryPort interface {
	Create(ctx context.Context, in proposaldomain.Proposal) (proposaldomain.Proposal, error)
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]proposaldomain.Proposal, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (proposaldomain.Proposal, error)
	UpdateDecision(ctx context.Context, orgID, id uuid.UUID, status proposaldomain.Status, decidedBy *string, decidedAt time.Time) (proposaldomain.Proposal, error)
	CreateVersion(ctx context.Context, orgID uuid.UUID, proposalID *uuid.UUID, label string, spec map[string]any, mode string, createdBy *string) error
}

type EventSink interface {
	Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error)
}

type Service interface {
	Create(ctx context.Context, orgID uuid.UUID, actor *string, req CreateRequest) (proposaldomain.Proposal, error)
	List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]proposaldomain.Proposal, error)
	Approve(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error)
	Reject(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error)
	Shadow(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error)
}

type CreateRequest struct {
	Status         string
	Diff           map[string]any
	Rationale      string
	TestsSuggested []string
	RollbackPlan   string
}

type service struct {
	repo   RepositoryPort
	events EventSink
}

func NewService(repo RepositoryPort, events EventSink) Service {
	return &service{repo: repo, events: events}
}

func (s *service) Create(ctx context.Context, orgID uuid.UUID, actor *string, req CreateRequest) (proposaldomain.Proposal, error) {
	st := proposaldomain.Status(req.Status)
	if st == "" {
		st = proposaldomain.StatusDraft
	}
	switch st {
	case proposaldomain.StatusDraft, proposaldomain.StatusPending:
	default:
		return proposaldomain.Proposal{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid status")
	}
	if req.Diff == nil {
		req.Diff = map[string]any{}
	}
	created, err := s.repo.Create(ctx, proposaldomain.Proposal{
		OrgID:          orgID,
		Status:         st,
		Diff:           req.Diff,
		Rationale:      req.Rationale,
		TestsSuggested: req.TestsSuggested,
		RollbackPlan:   req.RollbackPlan,
		CreatedBy:      actor,
	})
	if err != nil {
		return proposaldomain.Proposal{}, err
	}
	if s.events != nil {
		_, _ = s.events.Append(ctx, orgID, "proposal.created", map[string]any{
			"proposal_id": created.ID.String(),
			"status":      string(created.Status),
			"created_by":  created.CreatedBy,
			"diff":        created.Diff,
		})
	}
	return created, nil
}

func (s *service) List(ctx context.Context, orgID uuid.UUID, status string, limit int) ([]proposaldomain.Proposal, error) {
	return s.repo.List(ctx, orgID, status, limit)
}

func (s *service) Approve(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error) {
	return s.decide(ctx, orgID, id, actor, proposaldomain.StatusApproved, "proposal.approved", "enforced")
}

func (s *service) Reject(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error) {
	return s.decide(ctx, orgID, id, actor, proposaldomain.StatusRejected, "proposal.rejected", "")
}

func (s *service) Shadow(ctx context.Context, orgID, id uuid.UUID, actor *string) (proposaldomain.Proposal, error) {
	return s.decide(ctx, orgID, id, actor, proposaldomain.StatusShadow, "proposal.shadow_started", "shadow")
}

func (s *service) decide(ctx context.Context, orgID, id uuid.UUID, actor *string, status proposaldomain.Status, evType string, versionMode string) (proposaldomain.Proposal, error) {
	if _, err := s.repo.GetByID(ctx, orgID, id); err != nil {
		return proposaldomain.Proposal{}, err
	}
	now := time.Now().UTC()
	updated, err := s.repo.UpdateDecision(ctx, orgID, id, status, actor, now)
	if err != nil {
		return proposaldomain.Proposal{}, err
	}
	if versionMode != "" {
		label := string(status) + "-" + now.Format("20060102T150405Z")
		_ = s.repo.CreateVersion(ctx, orgID, &updated.ID, label, updated.Diff, versionMode, actor)
	}
	if s.events != nil {
		_, _ = s.events.Append(ctx, orgID, evType, map[string]any{
			"proposal_id": updated.ID.String(),
			"status":      string(updated.Status),
			"decided_by":  actor,
			"decided_at":  now.Format(time.RFC3339),
		})
	}
	return updated, nil
}
