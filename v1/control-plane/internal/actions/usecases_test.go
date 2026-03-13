package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	actiondomain "control-plane/internal/actions/usecases/domain"
	eventdomain "control-plane/internal/events/usecases/domain"
)

type stubActionRepo struct {
	created actiondomain.Action
	err     error
}

func (r *stubActionRepo) Create(_ context.Context, a actiondomain.Action) (actiondomain.Action, error) {
	if r.err != nil {
		return actiondomain.Action{}, r.err
	}
	a.ID = uuid.New()
	r.created = a
	return a, nil
}

func (r *stubActionRepo) GetByID(_ context.Context, _, id uuid.UUID) (actiondomain.Action, error) {
	return actiondomain.Action{ID: id, OrgID: uuid.New(), Status: actiondomain.StatusActive}, nil
}

func (r *stubActionRepo) List(_ context.Context, _ uuid.UUID, _, _ string, _ int) ([]actiondomain.Action, error) {
	return nil, nil
}

func (r *stubActionRepo) UpdateStatus(_ context.Context, orgID, id uuid.UUID, status actiondomain.Status, _ *string, _ *time.Time) (actiondomain.Action, error) {
	return actiondomain.Action{ID: id, OrgID: orgID, Status: status}, nil
}

func (r *stubActionRepo) ListExpiredCandidates(_ context.Context, _ time.Time, _ int) ([]actiondomain.Action, error) {
	return nil, nil
}

func (r *stubActionRepo) ListActiveForRun(_ context.Context, _ uuid.UUID, _ string, _ time.Time) ([]actiondomain.Action, error) {
	return nil, nil
}

type stubActionEventSink struct {
	calls int
}

func (s *stubActionEventSink) Append(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) (eventdomain.Event, error) {
	s.calls++
	return eventdomain.Event{}, nil
}

type stubActionMetering struct {
	calls   int
	counter string
}

func (s *stubActionMetering) Increment(_ context.Context, _ uuid.UUID, counter string) error {
	s.calls++
	s.counter = counter
	return nil
}

func validApplyReq() ApplyRequest {
	return ApplyRequest{
		ScopeType:  "tenant",
		ActionType: "throttle_tenant_rpm",
		TTLSeconds: 300,
		Params:     map[string]any{"per_minute": 10},
	}
}

func TestApply_CallsMetering(t *testing.T) {
	repo := &stubActionRepo{}
	events := &stubActionEventSink{}
	metering := &stubActionMetering{}
	svc := NewUsecases(repo, events, metering)

	_, err := svc.Apply(context.Background(), uuid.New(), nil, validApplyReq())
	if err != nil {
		t.Fatal(err)
	}

	if metering.calls != 1 {
		t.Errorf("expected 1 metering call, got %d", metering.calls)
	}
	if metering.counter != "actions_executed" {
		t.Errorf("expected actions_executed, got %s", metering.counter)
	}
}

func TestApply_NilMeteringDoesNotPanic(t *testing.T) {
	svc := NewUsecases(&stubActionRepo{}, nil, nil)
	_, err := svc.Apply(context.Background(), uuid.New(), nil, validApplyReq())
	if err != nil {
		t.Fatal(err)
	}
}

func TestApply_RepoError_DoesNotCallMetering(t *testing.T) {
	repo := &stubActionRepo{err: errors.New("db down")}
	metering := &stubActionMetering{}
	svc := NewUsecases(repo, nil, metering)

	_, err := svc.Apply(context.Background(), uuid.New(), nil, validApplyReq())
	if err == nil {
		t.Fatal("expected error")
	}
	if metering.calls != 0 {
		t.Errorf("metering should not be called on repo error, got %d calls", metering.calls)
	}
}

func TestRollback_DoesNotCallMetering(t *testing.T) {
	metering := &stubActionMetering{}
	svc := NewUsecases(&stubActionRepo{}, nil, metering)

	_, err := svc.Rollback(context.Background(), uuid.New(), uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if metering.calls != 0 {
		t.Errorf("Rollback should not call metering, got %d calls", metering.calls)
	}
}

func TestExpireDue_DoesNotCallMetering(t *testing.T) {
	metering := &stubActionMetering{}
	svc := NewUsecases(&stubActionRepo{}, nil, metering)

	_, err := svc.ExpireDue(context.Background(), time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if metering.calls != 0 {
		t.Errorf("ExpireDue should not call metering, got %d calls", metering.calls)
	}
}

func TestApply_InvalidScopeType_ReturnsError(t *testing.T) {
	svc := NewUsecases(&stubActionRepo{}, nil, nil)
	req := validApplyReq()
	req.ScopeType = "invalid"

	_, err := svc.Apply(context.Background(), uuid.New(), nil, req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestApply_InvalidActionType_ReturnsError(t *testing.T) {
	svc := NewUsecases(&stubActionRepo{}, nil, nil)
	req := validApplyReq()
	req.ActionType = "invalid"

	_, err := svc.Apply(context.Background(), uuid.New(), nil, req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
