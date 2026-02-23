package runtime_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"nexus-core/internal/agents/comms"
	"nexus-core/internal/agents/coordinator"
	"nexus-core/internal/agents/diagnostician"
	"nexus-core/internal/agents/mitigation"
	"nexus-core/internal/agents/recovery"
	"nexus-core/internal/agents/sentry"
	"nexus-core/internal/incidents"
	incidentdomain "nexus-core/internal/incidents/usecases/domain"
	opsaction "nexus-core/internal/ops/actionengine"
	actiondomain "nexus-core/internal/ops/actionengine/usecases/domain"
	commsdomain "nexus-core/internal/ops/comms/usecases/domain"
	diagnosisdomain "nexus-core/internal/ops/diagnosis/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	"nexus-core/internal/ops/llm"
)

func TestAgentFlowE2E_MockLLM(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	emitter := &queueEmitter{}

	sentryWorker := sentry.NewWorker(
		&sentryStateMem{},
		&incidentStub{},
		emitter,
		sentry.Config{Alpha: 1, ErrorThreshold: 0.2, MinSamples: 1},
	)
	diagnosisSvc := &diagnosisStub{}
	diagnosticianWorker := diagnostician.NewWorker(llmFlowStub{}, diagnosisSvc, emitter, "mock", "mock-default")
	commsSvc := &commsStub{}
	commsWorker := comms.NewWorker(llmFlowStub{}, commsSvc, emitter)
	engine := &engineFlowStub{emitter: emitter}
	mitigationWorker := mitigation.NewWorker(engine)
	recoveryWorker := recovery.NewWorker(emitter, recovery.Config{RequiredSuccesses: 1, MonitoringWindow: time.Nanosecond})
	coordWorker := coordinator.NewWorker(emitter)

	workers := []interface {
		Handle(context.Context, opsdomain.StoredEvent) error
	}{
		sentryWorker,
		coordWorker,
		diagnosticianWorker,
		mitigationWorker,
		recoveryWorker,
		commsWorker,
	}

	initial := opsdomain.StoredEvent{
		Sequence: 1,
		Envelope: opsdomain.Envelope{
			ID:         uuid.New(),
			EventType:  "tool_call.finished",
			Version:    1,
			OccurredAt: time.Now().UTC(),
			OrgID:      orgID,
			Correlation: opsdomain.Correlation{
				RequestID: ptr("req-initial"),
			},
			Actor:  opsdomain.Actor{ActorType: "system"},
			Source: "gateway.run",
			Payload: map[string]any{
				"tool_name": "world.move",
				"status":    "error",
			},
		},
	}

	queue := []opsdomain.StoredEvent{initial}
	processed := make([]opsdomain.StoredEvent, 0, 128)
	for i := 0; i < 200 && len(queue) > 0; i++ {
		ev := queue[0]
		queue = queue[1:]
		processed = append(processed, ev)
		for _, w := range workers {
			if err := w.Handle(context.Background(), ev); err != nil {
				t.Fatalf("worker handle failed for %s: %v", ev.Envelope.EventType, err)
			}
		}
		queue = append(queue, emitter.drain()...)
	}

	assertSeenType(t, processed, "anomaly.detected")
	assertSeenType(t, processed, "incident.opened")
	assertSeenType(t, processed, "diagnosis.created")
	assertSeenType(t, processed, "recommended_actions.created")
	assertSeenType(t, processed, "action.applied")
	assertSeenType(t, processed, "comms.draft_created")
	assertSeenType(t, processed, "incident.state_changed")

	if !hasStateTransition(processed, "RESOLVED") {
		t.Fatalf("expected incident.state_changed to RESOLVED")
	}
	if engine.dryRunCalls == 0 || engine.applyCalls == 0 {
		t.Fatalf("expected mitigation engine dry-run/apply calls")
	}
}

func assertSeenType(t *testing.T, events []opsdomain.StoredEvent, eventType string) {
	t.Helper()
	for _, ev := range events {
		if ev.Envelope.EventType == eventType {
			return
		}
	}
	t.Fatalf("expected event type %s", eventType)
}

func hasStateTransition(events []opsdomain.StoredEvent, toState string) bool {
	for _, ev := range events {
		if ev.Envelope.EventType != "incident.state_changed" {
			continue
		}
		if s, ok := ev.Envelope.Payload["to_state"].(string); ok && s == toState {
			return true
		}
	}
	return false
}

type queueEmitter struct {
	mu       sync.Mutex
	sequence int64
	queue    []opsdomain.StoredEvent
}

func (e *queueEmitter) Emit(_ context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sequence++
	ev := opsdomain.StoredEvent{
		Sequence: e.sequence,
		Envelope: opsdomain.Envelope{
			ID:          uuid.New(),
			EventType:   in.EventType,
			Version:     in.Version,
			OccurredAt:  time.Now().UTC(),
			OrgID:       in.OrgID,
			Correlation: in.Correlation,
			Actor:       in.Actor,
			Source:      in.Source,
			Payload:     in.Payload,
		},
		SchemaValid: true,
	}
	e.queue = append(e.queue, ev)
	return ev, nil
}

func (e *queueEmitter) drain() []opsdomain.StoredEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]opsdomain.StoredEvent, len(e.queue))
	copy(out, e.queue)
	e.queue = nil
	return out
}

type sentryStateMem struct {
	mu           sync.Mutex
	baselines    map[string]sentry.Baseline
	fingerprints map[string]sentry.FingerprintState
}

func (m *sentryStateMem) GetBaseline(_ context.Context, orgID uuid.UUID, toolName, metric string) (sentry.Baseline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.baselines == nil {
		m.baselines = map[string]sentry.Baseline{}
	}
	k := orgID.String() + "|" + toolName + "|" + metric
	return m.baselines[k], nil
}

func (m *sentryStateMem) UpsertBaseline(_ context.Context, b sentry.Baseline) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.baselines == nil {
		m.baselines = map[string]sentry.Baseline{}
	}
	k := b.OrgID.String() + "|" + b.ToolName + "|" + b.Metric
	m.baselines[k] = b
	return nil
}

func (m *sentryStateMem) GetFingerprint(_ context.Context, orgID uuid.UUID, fingerprint string) (sentry.FingerprintState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fingerprints == nil {
		m.fingerprints = map[string]sentry.FingerprintState{}
	}
	k := orgID.String() + "|" + fingerprint
	return m.fingerprints[k], nil
}

func (m *sentryStateMem) UpsertFingerprint(_ context.Context, f sentry.FingerprintState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fingerprints == nil {
		m.fingerprints = map[string]sentry.FingerprintState{}
	}
	k := f.OrgID.String() + "|" + f.Fingerprint
	m.fingerprints[k] = f
	return nil
}

type incidentStub struct{}

func (incidentStub) Create(_ context.Context, orgID uuid.UUID, actor *string, req incidents.CreateRequest) (incidentdomain.Incident, error) {
	_ = actor
	return incidentdomain.Incident{
		ID:       uuid.New(),
		OrgID:    orgID,
		Severity: incidentdomain.Severity(req.Severity),
		Status:   incidentdomain.StatusOpen,
		Title:    req.Title,
		Summary:  req.Summary,
	}, nil
}

type diagnosisStub struct{}

func (diagnosisStub) Create(_ context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error) {
	in.ID = uuid.New()
	in.CreatedAt = time.Now().UTC()
	return in, nil
}

func (diagnosisStub) ListByIncident(context.Context, uuid.UUID, uuid.UUID, int) ([]diagnosisdomain.Report, error) {
	return nil, nil
}

type commsStub struct{}

func (commsStub) Create(_ context.Context, in commsdomain.Draft) (commsdomain.Draft, error) {
	in.ID = uuid.New()
	in.CreatedAt = time.Now().UTC()
	return in, nil
}

func (commsStub) MarkStatus(context.Context, uuid.UUID, uuid.UUID, commsdomain.Status) (commsdomain.Draft, error) {
	return commsdomain.Draft{}, nil
}

func (commsStub) ListByIncident(context.Context, uuid.UUID, uuid.UUID, int) ([]commsdomain.Draft, error) {
	return nil, nil
}

type llmFlowStub struct{}

func (llmFlowStub) Generate(context.Context, llm.Request) (map[string]any, error) {
	return map[string]any{
		"answer":        "mock answer",
		"evidence_refs": []any{"incident:latest"},
	}, nil
}

func (llmFlowStub) GenerateStrict(_ context.Context, req llm.Request, schemaFile string) (map[string]any, error) {
	if schemaFile == "diagnosis_report.json" {
		return map[string]any{
			"summary_30s": "diagnosis generated",
			"root_cause":  "rate_limit_bucket_saturated",
			"confidence":  0.8,
			"evidence_refs": []any{
				"audit:req-1",
			},
			"hypotheses": []any{},
			"recommended_actions": []any{
				map[string]any{
					"action_type": "set_rate_limit",
					"scope": map[string]any{
						"level":   "tool",
						"org_id":  asString(req.Input["org_id"]),
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
	return map[string]any{
		"incident_id": asString(req.Input["incident_id"]),
		"summary":     "internal comms draft",
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
				"body":              "Mitigation applied and monitoring.",
				"approval_required": false,
			},
		},
	}, nil
}

type engineFlowStub struct {
	emitter          *queueEmitter
	dryRunCalls      int
	applyCalls       int
	proposalIncident map[string]string
}

func (e *engineFlowStub) DryRun(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	_ = ctx
	_ = orgID
	_ = actor
	e.dryRunCalls++
	if e.proposalIncident == nil {
		e.proposalIncident = map[string]string{}
	}
	proposalID := uuid.New()
	incidentID := ""
	if req.IncidentID != nil {
		incidentID = req.IncidentID.String()
	}
	e.proposalIncident[proposalID.String()] = incidentID
	return opsaction.EngineResult{
		Proposal: actiondomain.Proposal{
			ID:         proposalID,
			IncidentID: req.IncidentID,
			ActionType: req.ActionType,
		},
		ApprovalRequired: false,
	}, nil
}

func (e *engineFlowStub) Apply(ctx context.Context, orgID uuid.UUID, actor *string, req opsaction.EngineRequest) (opsaction.EngineResult, error) {
	e.applyCalls++
	incidentID := ""
	if req.ProposalID != nil {
		if e.proposalIncident != nil {
			incidentID = e.proposalIncident[req.ProposalID.String()]
		}
		_, _ = e.emitter.Emit(ctx, opseventstore.EmitInput{
			EventType: "action.applied",
			Version:   1,
			OrgID:     orgID,
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
			Actor:  opsdomain.Actor{ActorID: actor, ActorType: "agent"},
			Source: "engine.stub",
			Payload: map[string]any{
				"incident_id": incidentID,
				"proposal_id": req.ProposalID.String(),
				"action_type": "set_rate_limit",
			},
		})
		_, _ = e.emitter.Emit(ctx, opseventstore.EmitInput{
			EventType: "tool_call.finished",
			Version:   1,
			OrgID:     orgID,
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
			Actor:  opsdomain.Actor{ActorType: "system"},
			Source: "engine.stub",
			Payload: map[string]any{
				"incident_id": incidentID,
				"tool_name":   "world.move",
				"status":      "success",
			},
		})
	}
	return opsaction.EngineResult{}, nil
}

func (e *engineFlowStub) Rollback(context.Context, uuid.UUID, *string, opsaction.EngineRequest) (opsaction.EngineResult, error) {
	return opsaction.EngineResult{}, nil
}

func ptr(v string) *string {
	return &v
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
