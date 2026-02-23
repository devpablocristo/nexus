package recovery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

func TestRecoveryWorker_ResolvesAfterStableSuccesses(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	em := &emitBuffer{}
	now := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	w := NewWorker(em, Config{RequiredSuccesses: 2, MonitoringWindow: time.Nanosecond, Now: func() time.Time { return now }})

	if err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:      orgID,
			EventType:  "action.applied",
			OccurredAt: now,
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
		},
	}); err != nil {
		t.Fatalf("action.applied handling failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := w.Handle(context.Background(), opsdomain.StoredEvent{
			Envelope: opsdomain.Envelope{
				OrgID:      orgID,
				EventType:  "tool_call.finished",
				OccurredAt: now.Add(time.Duration(i+1) * time.Second),
				Correlation: opsdomain.Correlation{
					IncidentID: &incidentID,
				},
				Payload: map[string]any{"status": "success"},
			},
		}); err != nil {
			t.Fatalf("tool_call handling failed: %v", err)
		}
	}
	if !em.hasTransition("RESOLVED") {
		t.Fatalf("expected RESOLVED transition")
	}
}

func TestRecoveryWorker_AutoRollbackOnTTLExpiryOnIdle(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	current := time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)
	em := &emitBuffer{}
	w := NewWorker(em, Config{
		RequiredSuccesses: 2,
		MonitoringWindow:  10 * time.Minute,
		Now:               func() time.Time { return current },
	})

	if err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:      orgID,
			EventType:  "action.applied",
			OccurredAt: current,
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
			Payload: map[string]any{
				"ttl_seconds": 5,
				"action_id":   "act-1",
				"action_type": "set_rate_limit",
			},
		},
	}); err != nil {
		t.Fatalf("action.applied handling failed: %v", err)
	}

	current = current.Add(6 * time.Second)
	if err := w.OnIdle(context.Background()); err != nil {
		t.Fatalf("idle evaluation failed: %v", err)
	}

	if !em.hasType("action.rolled_back") {
		t.Fatalf("expected action.rolled_back on ttl expiry")
	}
	if !em.hasTransition("OPEN") {
		t.Fatalf("expected OPEN transition after ttl rollback")
	}
}

type emitBuffer struct {
	mu     sync.Mutex
	events []opseventstore.EmitInput
}

func (e *emitBuffer) Emit(_ context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, in)
	return opsdomain.StoredEvent{}, nil
}

func (e *emitBuffer) hasTransition(toState string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.events {
		if ev.EventType != "incident.state_changed" {
			continue
		}
		if s, ok := ev.Payload["to_state"].(string); ok && s == toState {
			return true
		}
	}
	return false
}

func (e *emitBuffer) hasType(eventType string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.events {
		if ev.EventType == eventType {
			return true
		}
	}
	return false
}
