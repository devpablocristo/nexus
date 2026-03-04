package incidents

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	eventdomain "nexus-saas/internal/events/usecases/domain"
	incidentdomain "nexus-saas/internal/incidents/usecases/domain"
)

type stubIncidentRepo struct {
	created incidentdomain.Incident
	err     error
}

func (r *stubIncidentRepo) Create(_ context.Context, in incidentdomain.Incident) (incidentdomain.Incident, error) {
	if r.err != nil {
		return incidentdomain.Incident{}, r.err
	}
	in.ID = uuid.New()
	r.created = in
	return in, nil
}

func (r *stubIncidentRepo) List(_ context.Context, _ uuid.UUID, _ string, _ int) ([]incidentdomain.Incident, error) {
	return nil, nil
}

func (r *stubIncidentRepo) GetByID(_ context.Context, _, id uuid.UUID) (incidentdomain.Incident, error) {
	return incidentdomain.Incident{ID: id}, nil
}

func (r *stubIncidentRepo) Close(_ context.Context, _, id uuid.UUID) (incidentdomain.Incident, error) {
	return incidentdomain.Incident{ID: id, Status: incidentdomain.StatusClosed}, nil
}

type stubEventSink struct {
	calls int
}

func (s *stubEventSink) Append(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) (eventdomain.Event, error) {
	s.calls++
	return eventdomain.Event{}, nil
}

type stubIncidentMetering struct {
	calls   int
	counter string
}

func (s *stubIncidentMetering) Increment(_ context.Context, _ uuid.UUID, counter string) error {
	s.calls++
	s.counter = counter
	return nil
}

func validCreateReq() CreateRequest {
	return CreateRequest{
		Severity: "HIGH",
		Title:    "Something broke",
		Summary:  "Details here",
	}
}

func TestCreate_CallsMetering(t *testing.T) {
	repo := &stubIncidentRepo{}
	events := &stubEventSink{}
	metering := &stubIncidentMetering{}
	svc := NewUsecases(repo, events, metering)

	_, err := svc.Create(context.Background(), uuid.New(), nil, validCreateReq())
	if err != nil {
		t.Fatal(err)
	}

	if metering.calls != 1 {
		t.Errorf("expected 1 metering call, got %d", metering.calls)
	}
	if metering.counter != "incidents_opened" {
		t.Errorf("expected incidents_opened, got %s", metering.counter)
	}
}

func TestCreate_NilMeteringDoesNotPanic(t *testing.T) {
	svc := NewUsecases(&stubIncidentRepo{}, nil, nil)
	_, err := svc.Create(context.Background(), uuid.New(), nil, validCreateReq())
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreate_RepoError_DoesNotCallMetering(t *testing.T) {
	repo := &stubIncidentRepo{err: errors.New("db down")}
	metering := &stubIncidentMetering{}
	svc := NewUsecases(repo, nil, metering)

	_, err := svc.Create(context.Background(), uuid.New(), nil, validCreateReq())
	if err == nil {
		t.Fatal("expected error")
	}
	if metering.calls != 0 {
		t.Errorf("metering should not be called on repo error, got %d calls", metering.calls)
	}
}

func TestCreate_InvalidSeverity_ReturnsError(t *testing.T) {
	svc := NewUsecases(&stubIncidentRepo{}, nil, nil)
	req := validCreateReq()
	req.Severity = "UNKNOWN"

	_, err := svc.Create(context.Background(), uuid.New(), nil, req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCreate_MissingTitle_ReturnsError(t *testing.T) {
	svc := NewUsecases(&stubIncidentRepo{}, nil, nil)
	req := validCreateReq()
	req.Title = ""

	_, err := svc.Create(context.Background(), uuid.New(), nil, req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}
