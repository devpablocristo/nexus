package session

import (
	"context"

	"github.com/google/uuid"

	domain "nexus-saas/internal/session/usecases/domain"
)

type RepoPort interface {
	GetOrCreate(ctx context.Context, orgID uuid.UUID, sessionID string, actor *string) (domain.AgentSession, error)
	IncrementCall(ctx context.Context, orgID uuid.UUID, sessionID string, isWrite bool, isDenial bool) error
	GetBySessionID(ctx context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error)
}

type Usecases struct {
	repo RepoPort
}

func NewUsecases(repo RepoPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) TrackCall(ctx context.Context, orgID uuid.UUID, sessionID string, actor *string, isWrite bool, isDenial bool) (domain.AgentSession, error) {
	sess, err := u.repo.GetOrCreate(ctx, orgID, sessionID, actor)
	if err != nil {
		return domain.AgentSession{}, err
	}
	if err := u.repo.IncrementCall(ctx, orgID, sessionID, isWrite, isDenial); err != nil {
		return sess, err
	}
	return sess, nil
}

func (u *Usecases) CheckLimits(sess domain.AgentSession, limits domain.SessionLimits) bool {
	if limits.MaxCallsPerSession > 0 && sess.TotalCalls >= limits.MaxCallsPerSession {
		return false
	}
	if limits.MaxWritesPerSession > 0 && sess.TotalWrites >= limits.MaxWritesPerSession {
		return false
	}
	return true
}

func (u *Usecases) GetBySessionID(ctx context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error) {
	return u.repo.GetBySessionID(ctx, orgID, sessionID)
}
