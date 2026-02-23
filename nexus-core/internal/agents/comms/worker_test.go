package comms

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	commsdomain "nexus-core/internal/ops/comms/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	"nexus-core/internal/ops/llm"
)

func TestCommsWorker_CreatesDraftAndEmitsInternalSend(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	svc := &commsStub{}
	emit := &emitStub{}
	w := NewWorker(llmCommsStub{}, svc, emit)

	err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:     orgID,
			EventType: "incident.opened",
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
			Payload: map[string]any{"incident_id": incidentID},
		},
	})
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if svc.calls == 0 {
		t.Fatalf("expected comms draft creation")
	}
	if !emit.hasType("comms.draft_created") {
		t.Fatalf("expected comms.draft_created")
	}
	if !emit.hasType("comms.sent_internal") {
		t.Fatalf("expected comms.sent_internal")
	}
}

type llmCommsStub struct{}

func (llmCommsStub) Generate(context.Context, llm.Request) (map[string]any, error) {
	return nil, nil
}

func (llmCommsStub) GenerateStrict(context.Context, llm.Request, string) (map[string]any, error) {
	return map[string]any{
		"incident_id": "f503f46f-c137-4165-b9ca-999d0d6f328f",
		"summary":     "mock comms summary",
		"root_cause_claim": map[string]any{
			"value":      "unknown",
			"confidence": 0.0,
		},
		"audiences": []any{
			map[string]any{"name": "oncall", "channel": "internal"},
		},
		"drafts": []any{
			map[string]any{
				"audience":          "oncall",
				"subject":           "Incident update",
				"body":              "Mitigacion temporal aplicada.",
				"approval_required": false,
			},
		},
	}, nil
}

type commsStub struct {
	calls int
}

func (c *commsStub) Create(_ context.Context, in commsdomain.Draft) (commsdomain.Draft, error) {
	c.calls++
	in.ID = uuid.New()
	in.CreatedAt = time.Now().UTC()
	return in, nil
}

func (c *commsStub) MarkStatus(context.Context, uuid.UUID, uuid.UUID, commsdomain.Status) (commsdomain.Draft, error) {
	return commsdomain.Draft{}, nil
}

func (c *commsStub) ListByIncident(context.Context, uuid.UUID, uuid.UUID, int) ([]commsdomain.Draft, error) {
	return nil, nil
}

type emitStub struct {
	mu    sync.Mutex
	types []string
}

func (e *emitStub) Emit(_ context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.types = append(e.types, in.EventType)
	return opsdomain.StoredEvent{}, nil
}

func (e *emitStub) hasType(v string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.types {
		if t == v {
			return true
		}
	}
	return false
}
