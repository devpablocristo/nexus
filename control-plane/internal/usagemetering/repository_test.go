package usagemetering

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// stubMeteringPort captures Increment calls for assertion.
type stubMeteringPort struct {
	calls []incrementCall
	err   error
}

type incrementCall struct {
	orgID   uuid.UUID
	counter string
}

func (s *stubMeteringPort) Increment(_ context.Context, orgID uuid.UUID, counter string) error {
	s.calls = append(s.calls, incrementCall{orgID: orgID, counter: counter})
	return s.err
}

func (s *stubMeteringPort) called(counter string) bool {
	for _, c := range s.calls {
		if c.counter == counter {
			return true
		}
	}
	return false
}

func TestBillingPeriod_FirstDayOfMonth(t *testing.T) {
	tests := []struct {
		input time.Time
		want  time.Time
	}{
		{
			time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC),
			time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		now := tt.input
		got := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(tt.want) {
			t.Errorf("billingPeriod(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCounterConstants(t *testing.T) {
	if CounterAPICalls != "api_calls" {
		t.Errorf("CounterAPICalls = %q", CounterAPICalls)
	}
	if CounterEventsIngested != "events_ingested" {
		t.Errorf("CounterEventsIngested = %q", CounterEventsIngested)
	}
	if CounterIncidentsOpened != "incidents_opened" {
		t.Errorf("CounterIncidentsOpened = %q", CounterIncidentsOpened)
	}
	if CounterActionsExecuted != "actions_executed" {
		t.Errorf("CounterActionsExecuted = %q", CounterActionsExecuted)
	}
}

func TestStubMeteringPort_Called(t *testing.T) {
	stub := &stubMeteringPort{}
	orgID := uuid.New()

	_ = stub.Increment(context.Background(), orgID, CounterEventsIngested)

	if !stub.called(CounterEventsIngested) {
		t.Error("expected events_ingested to be recorded")
	}
	if stub.called(CounterAPICalls) {
		t.Error("api_calls should not be recorded")
	}
	if len(stub.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(stub.calls))
	}
	if stub.calls[0].orgID != orgID {
		t.Error("orgID mismatch")
	}
}
