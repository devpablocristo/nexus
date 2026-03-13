package approval

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	domain "nexus/v2/data-plane/internal/approval/usecases/domain"
)

type RepoPort interface {
	Create(ctx context.Context, req domain.CreateRequest) (domain.PendingApproval, error)
	ListPending(ctx context.Context, limit int) ([]domain.PendingApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.PendingApproval, error)
	Decide(ctx context.Context, id uuid.UUID, status domain.Status, decidedBy string) (domain.PendingApproval, error)
}

type IntentStatusPort interface {
	MarkApproved(ctx context.Context, intentID uuid.UUID) error
	MarkRejected(ctx context.Context, intentID uuid.UUID) error
}

type Usecases struct {
	repo    RepoPort
	intents IntentStatusPort
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

func (u *Usecases) ListPending(ctx context.Context, limit int) ([]domain.PendingApproval, error) {
	return u.repo.ListPending(ctx, limit)
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (domain.PendingApproval, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return domain.PendingApproval{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) Approve(ctx context.Context, id uuid.UUID, decidedBy string) (domain.PendingApproval, error) {
	item, err := u.repo.Decide(ctx, id, domain.StatusApproved, decidedBy)
	if err != nil {
		return domain.PendingApproval{}, mapRepoErr(err)
	}
	if item.IntentID != nil && u.intents != nil {
		if err := u.intents.MarkApproved(ctx, *item.IntentID); err != nil {
			return domain.PendingApproval{}, err
		}
	}
	return item, nil
}

func (u *Usecases) Reject(ctx context.Context, id uuid.UUID, decidedBy string) (domain.PendingApproval, error) {
	item, err := u.repo.Decide(ctx, id, domain.StatusRejected, decidedBy)
	if err != nil {
		return domain.PendingApproval{}, mapRepoErr(err)
	}
	if item.IntentID != nil && u.intents != nil {
		if err := u.intents.MarkRejected(ctx, *item.IntentID); err != nil {
			return domain.PendingApproval{}, err
		}
	}
	return item, nil
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string {
	return e.Message
}

func newHTTPError(status int, code, message string) error {
	return httpError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "approval not found")
	}
	if errors.Is(err, ErrAlreadyDecided) {
		return newHTTPError(http.StatusConflict, "ALREADY_DECIDED", "approval already decided")
	}
	return err
}
