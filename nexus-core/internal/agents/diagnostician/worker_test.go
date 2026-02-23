package diagnostician

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	diagnosisdomain "nexus-core/internal/ops/diagnosis/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	"nexus-core/internal/ops/llm"
)

func TestDiagnosticianWorker_EmitsDiagnosisAndRecommendations(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	incidentID := uuid.MustParse("f503f46f-c137-4165-b9ca-999d0d6f328f")

	diagSvc := &diagStub{}
	emitter := &emitRecorder{}
	w := NewWorker(llmStub{}, diagSvc, emitter, "mock", "mock-default")

	err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:     orgID,
			EventType: "incident.opened",
			Correlation: opsdomain.Correlation{
				IncidentID: ptrStr(incidentID.String()),
			},
			Payload: map[string]any{"incident_id": incidentID.String()},
		},
	})
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if diagSvc.calls != 1 {
		t.Fatalf("expected diagnosis create call")
	}
	if !emitter.hasType("diagnosis.created") {
		t.Fatalf("expected diagnosis.created event")
	}
	if !emitter.hasType("recommended_actions.created") {
		t.Fatalf("expected recommended_actions.created event")
	}
}

type llmStub struct{}

func (llmStub) Generate(context.Context, llm.Request) (map[string]any, error) {
	return nil, nil
}

func (llmStub) GenerateStrict(context.Context, llm.Request, string) (map[string]any, error) {
	return map[string]any{
		"summary_30s": "mock diagnosis",
		"root_cause":  "rate_limit_bucket_saturated",
		"confidence":  0.9,
		"evidence_refs": []any{
			"audit:req-1",
		},
		"hypotheses": []any{},
		"recommended_actions": []any{
			map[string]any{
				"action_type": "set_rate_limit",
				"scope": map[string]any{
					"level":   "tool",
					"org_id":  "996e9e43-7bab-4e68-a831-0a766befbf54",
					"tool_id": "world.move",
				},
				"ttl_seconds": 600,
				"params": map[string]any{
					"rpm":     180,
					"tool_id": "world.move",
				},
			},
		},
		"missing_info": []any{},
	}, nil
}

type diagStub struct {
	calls int
}

func (d *diagStub) Create(_ context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error) {
	d.calls++
	in.ID = uuid.New()
	in.CreatedAt = time.Now().UTC()
	return in, nil
}

func (d *diagStub) ListByIncident(context.Context, uuid.UUID, uuid.UUID, int) ([]diagnosisdomain.Report, error) {
	return nil, nil
}

type emitRecorder struct {
	mu    sync.Mutex
	types []string
}

func (e *emitRecorder) Emit(_ context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.types = append(e.types, in.EventType)
	return opsdomain.StoredEvent{}, nil
}

func (e *emitRecorder) hasType(v string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.types {
		if t == v {
			return true
		}
	}
	return false
}

func ptrStr(v string) *string {
	return &v
}
