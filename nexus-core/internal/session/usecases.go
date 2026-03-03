package session

import (
	"context"

	"github.com/google/uuid"

	domain "nexus-core/internal/session/usecases/domain"
)

type RepoPort interface {
	GetOrCreate(ctx context.Context, orgID uuid.UUID, sessionID string, actor *string) (domain.AgentSession, error)
	IncrementCall(ctx context.Context, orgID uuid.UUID, sessionID string, isWrite bool, isDenial bool) error
	GetBySessionID(ctx context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error)
}

type Service struct {
	repo RepoPort
}

func NewService(repo RepoPort) *Service {
	return &Service{repo: repo}
}

// TrackCall upserts the session and increments the call counter.
// Returns the session state BEFORE incrementing (for limit checks).
func (s *Service) TrackCall(ctx context.Context, orgID uuid.UUID, sessionID string, actor *string, isWrite bool, isDenial bool) (domain.AgentSession, error) {
	sess, err := s.repo.GetOrCreate(ctx, orgID, sessionID, actor)
	if err != nil {
		return domain.AgentSession{}, err
	}
	if err := s.repo.IncrementCall(ctx, orgID, sessionID, isWrite, isDenial); err != nil {
		return sess, err
	}
	return sess, nil
}

// CheckLimits returns true if the session is within limits.
func (s *Service) CheckLimits(sess domain.AgentSession, limits domain.SessionLimits) bool {
	if limits.MaxCallsPerSession > 0 && sess.TotalCalls >= limits.MaxCallsPerSession {
		return false
	}
	if limits.MaxWritesPerSession > 0 && sess.TotalWrites >= limits.MaxWritesPerSession {
		return false
	}
	return true
}

func (s *Service) GetBySessionID(ctx context.Context, orgID uuid.UUID, sessionID string) (domain.AgentSession, error) {
	return s.repo.GetBySessionID(ctx, orgID, sessionID)
}
