package coordinator

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Worker struct {
	emitter EventEmitter
	mu      sync.Mutex
	states  map[string]string
}

func NewWorker(emitter EventEmitter) *Worker {
	return &Worker{
		emitter: emitter,
		states:  map[string]string{},
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.coordinator.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	incidentID := resolveIncidentID(event)
	if incidentID == "" {
		return nil
	}
	var target string
	switch event.Envelope.EventType {
	case "incident.opened":
		target = "DIAGNOSING"
	case "diagnosis.created", "recommended_actions.created":
		target = "MITIGATING"
	case "action.applied":
		target = "MONITORING"
	case "action.failed":
		target = "ESCALATED"
	case "action.rolled_back":
		target = "OPEN"
	default:
		return nil
	}

	w.mu.Lock()
	current := w.states[incidentID]
	if current == "" {
		current = "OPEN"
	}
	if current == target {
		w.mu.Unlock()
		return nil
	}
	w.states[incidentID] = target
	w.mu.Unlock()

	return w.emitStateChange(ctx, event.Envelope.OrgID, incidentID, current, target, "coordinator_transition")
}

func (w *Worker) emitStateChange(ctx context.Context, orgID uuid.UUID, incidentID, fromState, toState, reason string) error {
	if w.emitter == nil {
		return nil
	}
	actorID := "agents.coordinator"
	incID := incidentID
	_, err := w.emitter.Emit(ctx, opseventstore.EmitInput{
		EventType: "incident.state_changed",
		Version:   1,
		OrgID:     orgID,
		Correlation: opsdomain.Correlation{
			IncidentID: &incID,
		},
		Actor: opsdomain.Actor{
			ActorID:   &actorID,
			ActorType: "agent",
		},
		Source: "agents.coordinator",
		Payload: map[string]any{
			"incident_id": incidentID,
			"from_state":  fromState,
			"to_state":    toState,
			"reason":      reason,
		},
	})
	return err
}

func resolveIncidentID(event opsdomain.StoredEvent) string {
	if event.Envelope.Correlation.IncidentID != nil && *event.Envelope.Correlation.IncidentID != "" {
		return *event.Envelope.Correlation.IncidentID
	}
	if s := strings.TrimSpace(asString(event.Envelope.Payload["incident_id"])); s != "" {
		return s
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
