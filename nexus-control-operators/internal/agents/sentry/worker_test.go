package sentry

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-control-operators/internal/incidents"
	incidentdomain "nexus-control-operators/internal/incidents/usecases/domain"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

func TestSentryWorker_EmitsAnomalyAndIncident(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	state := newMemState()
	inc := &incidentStub{}
	emitter := &emitStub{}
	w := NewWorker(state, inc, emitter, Config{
		Alpha:          1.0,
		ErrorThreshold: 0.5,
		MinSamples:     1,
	}, zerolog.Nop())

	err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Sequence: 12,
		Envelope: opsdomain.Envelope{
			OrgID:      orgID,
			EventType:  "tool_call.finished",
			OccurredAt: time.Now().UTC(),
			Payload: map[string]any{
				"tool_name": "echo",
				"status":    "error",
			},
		},
	})
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}

	if inc.calls != 1 {
		t.Fatalf("expected one incident creation, got=%d", inc.calls)
	}
	if !emitter.hasType("anomaly.detected") {
		t.Fatalf("expected anomaly.detected event")
	}
	if !emitter.hasType("incident.opened") {
		t.Fatalf("expected incident.opened event")
	}
}

func TestSentryWorker_ConsumesPolicyQuotaAndDegradedSignals(t *testing.T) {
	t.Parallel()

	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	state := newMemState()
	inc := &incidentStub{}
	emitter := &emitStub{}
	w := NewWorker(state, inc, emitter, Config{
		Alpha:          1.0,
		ErrorThreshold: 0.5,
		MinSamples:     5,
		P95LatencyMS:   1200,
	}, zerolog.Nop())

	cases := []opsdomain.StoredEvent{
		{
			Sequence: 21,
			Envelope: opsdomain.Envelope{
				OrgID:      orgID,
				EventType:  "policy.denied",
				OccurredAt: time.Now().UTC(),
				Payload: map[string]any{
					"tool_name": "echo",
					"policy_id": "policy.low_rate",
				},
			},
		},
		{
			Sequence: 22,
			Envelope: opsdomain.Envelope{
				OrgID:      orgID,
				EventType:  "quota.exceeded",
				OccurredAt: time.Now().UTC(),
				Payload: map[string]any{
					"tool_name": "echo",
					"bucket":    "org:echo",
				},
			},
		},
		{
			Sequence: 23,
			Envelope: opsdomain.Envelope{
				OrgID:      orgID,
				EventType:  "tool_degraded",
				OccurredAt: time.Now().UTC(),
				Payload: map[string]any{
					"tool_name":        "echo",
					"degradation_type": "p95_latency",
					"p95_latency_ms":   1800,
				},
			},
		},
	}
	for _, ev := range cases {
		if err := w.Handle(context.Background(), ev); err != nil {
			t.Fatalf("handle failed for %s: %v", ev.Envelope.EventType, err)
		}
	}

	if inc.calls == 0 {
		t.Fatalf("expected at least one incident creation")
	}
	if !emitter.hasType("anomaly.detected") {
		t.Fatalf("expected anomaly.detected emission")
	}
}

type memState struct {
	mu           sync.Mutex
	baselines    map[string]Baseline
	fingerprints map[string]FingerprintState
}

func newMemState() *memState {
	return &memState{
		baselines:    map[string]Baseline{},
		fingerprints: map[string]FingerprintState{},
	}
}

func key(orgID uuid.UUID, tool, metric string) string {
	return orgID.String() + "|" + tool + "|" + metric
}

func (m *memState) GetBaseline(_ context.Context, orgID uuid.UUID, toolName, metric string) (Baseline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.baselines[key(orgID, toolName, metric)]; ok {
		return b, nil
	}
	return Baseline{OrgID: orgID, ToolName: toolName, Metric: metric}, nil
}

func (m *memState) UpsertBaseline(_ context.Context, b Baseline) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baselines[key(b.OrgID, b.ToolName, b.Metric)] = b
	return nil
}

func (m *memState) GetFingerprint(_ context.Context, orgID uuid.UUID, fingerprint string) (FingerprintState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f, ok := m.fingerprints[orgID.String()+"|"+fingerprint]; ok {
		return f, nil
	}
	return FingerprintState{OrgID: orgID, Fingerprint: fingerprint}, nil
}

func (m *memState) UpsertFingerprint(_ context.Context, f FingerprintState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fingerprints[f.OrgID.String()+"|"+f.Fingerprint] = f
	return nil
}

type incidentStub struct {
	calls int
}

func (i *incidentStub) Create(_ context.Context, orgID uuid.UUID, actor *string, req incidents.CreateRequest) (incidentdomain.Incident, error) {
	i.calls++
	return incidentdomain.Incident{
		ID:       uuid.New(),
		OrgID:    orgID,
		Severity: incidentdomain.Severity(req.Severity),
		Title:    req.Title,
		Summary:  req.Summary,
	}, nil
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

func (e *emitStub) hasType(typ string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.types {
		if t == typ {
			return true
		}
	}
	return false
}
