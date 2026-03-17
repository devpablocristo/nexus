package approvals

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	approvaldomain "github.com/devpablocristo/nexus/review-v1/internal/approvals/usecases/domain"
	requestdomain "github.com/devpablocristo/nexus/review-v1/internal/requests/usecases/domain"
)

type RequestUpdater interface {
	GetByID(ctx context.Context, id uuid.UUID) (requestdomain.Request, error)
	Update(ctx context.Context, r requestdomain.Request) (requestdomain.Request, error)
}

type Usecases struct {
	repo        Repository
	requestRepo RequestUpdater
}

func NewUsecases(repo Repository, requestRepo RequestUpdater) *Usecases {
	return &Usecases{repo: repo, requestRepo: requestRepo}
}

func (u *Usecases) ListPending(ctx context.Context, limit int) ([]approvaldomain.Approval, error) {
	return u.repo.ListPending(ctx, limit)
}

func (u *Usecases) Approve(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return err
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}
	now := time.Now().UTC()
	a.Status = approvaldomain.ApprovalStatusApproved
	a.DecidedBy = decidedBy
	a.DecisionNote = note
	a.DecidedAt = &now
	_, err = u.repo.Update(ctx, a)
	if err != nil {
		return err
	}
	req, err := u.requestRepo.GetByID(ctx, a.RequestID)
	if err != nil {
		return err
	}
	req.Status = requestdomain.StatusApproved
	req.DecidedAt = &now
	req.UpdatedAt = now
	_, err = u.requestRepo.Update(ctx, req)
	return err
}

func (u *Usecases) Reject(ctx context.Context, approvalID uuid.UUID, decidedBy, note string) error {
	a, err := u.repo.GetByID(ctx, approvalID)
	if err != nil {
		return err
	}
	if a.Status != approvaldomain.ApprovalStatusPending {
		return ErrNotPending
	}
	now := time.Now().UTC()
	a.Status = approvaldomain.ApprovalStatusRejected
	a.DecidedBy = decidedBy
	a.DecisionNote = note
	a.DecidedAt = &now
	_, err = u.repo.Update(ctx, a)
	if err != nil {
		return err
	}
	req, err := u.requestRepo.GetByID(ctx, a.RequestID)
	if err != nil {
		return err
	}
	req.Status = requestdomain.StatusRejected
	req.DecidedAt = &now
	req.UpdatedAt = now
	_, err = u.requestRepo.Update(ctx, req)
	return err
}

var ErrNotPending = errors.New("approval is not pending")
