package coordinator

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

func TestCoordinatorWorker_TransitionsIncidentState(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	em := &emitRecorder{}
	w := NewWorker(em, zerolog.Nop())

	for _, evType := range []string{"incident.opened", "diagnosis.created", "action.applied"} {
		if err := w.Handle(context.Background(), opsdomain.StoredEvent{
			Envelope: opsdomain.Envelope{
				OrgID:     orgID,
				EventType: evType,
				Correlation: opsdomain.Correlation{
					IncidentID: &incidentID,
				},
				Payload: map[string]any{
					"incident_id": incidentID,
				},
			},
		}); err != nil {
			t.Fatalf("handle(%s) failed: %v", evType, err)
		}
	}

	if !em.hasType("incident.state_changed") {
		t.Fatalf("expected incident.state_changed emission")
	}
	if got := em.lastToState(); got != "MONITORING" {
		t.Fatalf("unexpected final state: got=%s want=MONITORING", got)
	}
}

type emitRecorder struct {
	mu     sync.Mutex
	events []opseventstore.EmitInput
}

func (e *emitRecorder) Emit(_ context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, in)
	return opsdomain.StoredEvent{}, nil
}

func (e *emitRecorder) hasType(eventType string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ev := range e.events {
		if ev.EventType == eventType {
			return true
		}
	}
	return false
}

func (e *emitRecorder) lastToState() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.events) - 1; i >= 0; i-- {
		if e.events[i].EventType != "incident.state_changed" {
			continue
		}
		if v, ok := e.events[i].Payload["to_state"].(string); ok {
			return v
		}
	}
	return ""
}
