package recovery

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

func TestRecoveryWorker_ResolvesAfterStableSuccesses(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	em := &emitBuffer{}
	w := NewWorker(em, Config{RequiredSuccesses: 2})

	if err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:     orgID,
			EventType: "action.applied",
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
				OrgID:     orgID,
				EventType: "tool_call.finished",
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
