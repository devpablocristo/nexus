package events

import (
	"context"

	"github.com/google/uuid"

	eventdomain "nexus-core/internal/events/usecases/domain"
)

type RepositoryPort interface {
	Create(ctx context.Context, ev eventdomain.Event) (eventdomain.Event, error)
	ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, error)
}

type Service interface {
	Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error)
	ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, int64, error)
}

type service struct {
	repo RepositoryPort
}

func NewService(repo RepositoryPort) Service {
	return &service{repo: repo}
}

func (s *service) Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	return s.repo.Create(ctx, eventdomain.Event{
		OrgID:     orgID,
		EventType: eventType,
		Payload:   payload,
	})
}

func (s *service) ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, int64, error) {
	items, err := s.repo.ListByCursor(ctx, orgID, cursor, limit)
	if err != nil {
		return nil, 0, err
	}
	next := cursor
	if len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return items, next, nil
}
