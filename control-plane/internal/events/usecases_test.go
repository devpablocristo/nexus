package events

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	eventdomain "control-plane/internal/events/usecases/domain"
)

// stubRepo implements RepositoryPort.
type stubRepo struct {
	created eventdomain.Event
	err     error
}

func (r *stubRepo) Create(_ context.Context, ev eventdomain.Event) (eventdomain.Event, error) {
	if r.err != nil {
		return eventdomain.Event{}, r.err
	}
	ev.ID = 1
	r.created = ev
	return ev, nil
}

func (r *stubRepo) ListByCursor(_ context.Context, _ uuid.UUID, _ int64, _ int) ([]eventdomain.Event, error) {
	return nil, nil
}

func (r *stubRepo) ListRecent(_ context.Context, _ uuid.UUID, _ int) ([]eventdomain.Event, error) {
	return nil, nil
}

// stubMetering implements MeteringPort.
type stubMetering struct {
	calls   int
	counter string
}

func (s *stubMetering) Increment(_ context.Context, _ uuid.UUID, counter string) error {
	s.calls++
	s.counter = counter
	return nil
}

func TestAppend_CallsMetering(t *testing.T) {
	repo := &stubRepo{}
	metering := &stubMetering{}
	svc := NewUsecases(repo, metering)

	orgID := uuid.New()
	_, err := svc.Append(context.Background(), orgID, "tool.called", nil)
	if err != nil {
		t.Fatal(err)
	}

	if metering.calls != 1 {
		t.Errorf("expected 1 metering call, got %d", metering.calls)
	}
	if metering.counter != "events_ingested" {
		t.Errorf("expected counter events_ingested, got %s", metering.counter)
	}
}

func TestAppend_NilMeteringDoesNotPanic(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo, nil)

	_, err := svc.Append(context.Background(), uuid.New(), "tool.called", nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppend_RepoError_DoesNotCallMetering(t *testing.T) {
	repo := &stubRepo{err: errors.New("db down")}
	metering := &stubMetering{}
	svc := NewUsecases(repo, metering)

	_, err := svc.Append(context.Background(), uuid.New(), "tool.called", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if metering.calls != 0 {
		t.Errorf("metering should not be called on repo error, got %d calls", metering.calls)
	}
}

func TestAppend_NilPayloadDefaultsToEmpty(t *testing.T) {
	repo := &stubRepo{}
	svc := NewUsecases(repo, nil)

	_, err := svc.Append(context.Background(), uuid.New(), "tool.called", nil)
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.Payload == nil {
		t.Error("payload should default to empty map, not nil")
	}
}
