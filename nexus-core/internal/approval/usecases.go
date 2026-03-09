package approval

import (
	"context"

	"github.com/google/uuid"

	domain "nexus-core/internal/approval/usecases/domain"
)

type RepoPort interface {
	Create(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error)
	ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error)
	Decide(ctx context.Context, orgID, id uuid.UUID, status domain.Status, decidedBy string) error
	ExpireOld(ctx context.Context) (int64, error)
}

type Usecases struct {
	repo    RepoPort
	intents IntentStatusPort
}

type IntentStatusPort interface {
	MarkIntentApproved(ctx context.Context, orgID, intentID uuid.UUID) error
	MarkIntentRejected(ctx context.Context, orgID, intentID uuid.UUID) error
}

func NewUsecases(repo RepoPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) WithIntentPort(port IntentStatusPort) *Usecases {
	u.intents = port
	return u
}

func (u *Usecases) RequestApproval(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error) {
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 3600
	}
	return u.repo.Create(ctx, req)
}

func (u *Usecases) ListPending(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.PendingApproval, error) {
	return u.repo.ListPending(ctx, orgID, limit)
}

func (u *Usecases) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.PendingApproval, error) {
	return u.repo.GetByID(ctx, orgID, id)
}

func (u *Usecases) Approve(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if err := u.repo.Decide(ctx, orgID, id, domain.StatusApproved, decidedBy); err != nil {
		return err
	}
	if item.IntentID != nil && u.intents != nil {
		return u.intents.MarkIntentApproved(ctx, orgID, *item.IntentID)
	}
	return nil
}

func (u *Usecases) Reject(ctx context.Context, orgID, id uuid.UUID, decidedBy string) error {
	item, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if err := u.repo.Decide(ctx, orgID, id, domain.StatusRejected, decidedBy); err != nil {
		return err
	}
	if item.IntentID != nil && u.intents != nil {
		return u.intents.MarkIntentRejected(ctx, orgID, *item.IntentID)
	}
	return nil
}

func (u *Usecases) ExpireOld(ctx context.Context) (int64, error) {
	return u.repo.ExpireOld(ctx)
}
