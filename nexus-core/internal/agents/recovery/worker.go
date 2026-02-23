package recovery

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

type Config struct {
	RequiredSuccesses int
}

type Worker struct {
	emitter           EventEmitter
	requiredSuccesses int
	mu                sync.Mutex
	successByIncident map[string]int
}

func NewWorker(emitter EventEmitter, cfg Config) *Worker {
	required := cfg.RequiredSuccesses
	if required <= 0 {
		required = 3
	}
	return &Worker{
		emitter:           emitter,
		requiredSuccesses: required,
		successByIncident: map[string]int{},
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.recovery.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	incidentID := resolveIncidentID(event)
	if incidentID == "" {
		return nil
	}
	switch event.Envelope.EventType {
	case "action.applied":
		w.mu.Lock()
		w.successByIncident[incidentID] = 0
		w.mu.Unlock()
		return w.emitState(ctx, event.Envelope.OrgID, incidentID, "MITIGATING", "MONITORING", "post_apply_monitoring")
	case "tool_call.finished":
		status := strings.ToLower(strings.TrimSpace(asString(event.Envelope.Payload["status"])))
		w.mu.Lock()
		count, tracked := w.successByIncident[incidentID]
		w.mu.Unlock()
		if !tracked {
			return nil
		}
		if status == "success" {
			count++
			w.mu.Lock()
			w.successByIncident[incidentID] = count
			w.mu.Unlock()
			if count >= w.requiredSuccesses {
				w.mu.Lock()
				delete(w.successByIncident, incidentID)
				w.mu.Unlock()
				return w.emitState(ctx, event.Envelope.OrgID, incidentID, "MONITORING", "RESOLVED", "stable_after_mitigation")
			}
			return nil
		}
		w.mu.Lock()
		delete(w.successByIncident, incidentID)
		w.mu.Unlock()
		if err := w.emitActionRollback(ctx, event.Envelope.OrgID, incidentID); err != nil {
			return err
		}
		return w.emitState(ctx, event.Envelope.OrgID, incidentID, "MONITORING", "OPEN", "regressed_after_mitigation")
	default:
		return nil
	}
}

func (w *Worker) emitState(ctx context.Context, orgID uuid.UUID, incidentID, fromState, toState, reason string) error {
	if w.emitter == nil {
		return nil
	}
	incID := incidentID
	actorID := "agents.recovery"
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
		Source: "agents.recovery",
		Payload: map[string]any{
			"incident_id": incidentID,
			"from_state":  fromState,
			"to_state":    toState,
			"reason":      reason,
		},
	})
	return err
}

func (w *Worker) emitActionRollback(ctx context.Context, orgID uuid.UUID, incidentID string) error {
	if w.emitter == nil {
		return nil
	}
	incID := incidentID
	actorID := "agents.recovery"
	_, err := w.emitter.Emit(ctx, opseventstore.EmitInput{
		EventType: "action.rolled_back",
		Version:   1,
		OrgID:     orgID,
		Correlation: opsdomain.Correlation{
			IncidentID: &incID,
		},
		Actor: opsdomain.Actor{
			ActorID:   &actorID,
			ActorType: "agent",
		},
		Source: "agents.recovery",
		Payload: map[string]any{
			"incident_id": incidentID,
			"reason":      "post_mitigation_regression",
		},
	})
	return err
}

func resolveIncidentID(event opsdomain.StoredEvent) string {
	if event.Envelope.Correlation.IncidentID != nil && *event.Envelope.Correlation.IncidentID != "" {
		return *event.Envelope.Correlation.IncidentID
	}
	if raw := strings.TrimSpace(asString(event.Envelope.Payload["incident_id"])); raw != "" {
		return raw
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
