package events

import (
	"context"

	"github.com/google/uuid"

	eventdomain "nexus-saas/internal/events/usecases/domain"
)

type RepositoryPort interface {
	Create(ctx context.Context, ev eventdomain.Event) (eventdomain.Event, error)
	ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, error)
	ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]eventdomain.Event, error)
}

type MeteringPort interface {
	Increment(ctx context.Context, orgID uuid.UUID, counter string) error
}

type Usecases struct {
	repo     RepositoryPort
	metering MeteringPort
}

func NewUsecases(repo RepositoryPort, metering MeteringPort) *Usecases {
	return &Usecases{repo: repo, metering: metering}
}

func (u *Usecases) Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	ev, err := u.repo.Create(ctx, eventdomain.Event{
		OrgID:     orgID,
		EventType: eventType,
		Payload:   payload,
	})
	if err != nil {
		return eventdomain.Event{}, err
	}
	if u.metering != nil {
		_ = u.metering.Increment(ctx, orgID, "events_ingested")
	}
	return ev, nil
}

func (u *Usecases) ListByCursor(ctx context.Context, orgID uuid.UUID, cursor int64, limit int) ([]eventdomain.Event, int64, error) {
	items, err := u.repo.ListByCursor(ctx, orgID, cursor, limit)
	if err != nil {
		return nil, 0, err
	}
	next := cursor
	if len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return items, next, nil
}

func (u *Usecases) ListRecent(ctx context.Context, orgID uuid.UUID, limit int) ([]eventdomain.Event, error) {
	return u.repo.ListRecent(ctx, orgID, limit)
}
