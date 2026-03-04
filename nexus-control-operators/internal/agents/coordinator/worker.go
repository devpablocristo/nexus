package coordinator

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-control-operators/internal/agents/coordinator/worker"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
	"nexus-control-operators/internal/shared/eventutil"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Worker struct {
	emitter EventEmitter
	log     zerolog.Logger
	mu      sync.Mutex
	states  map[string]string
}

func NewWorker(emitter EventEmitter, log zerolog.Logger) *Worker {
	return &Worker{
		emitter: emitter,
		log:     log.With().Str("worker", "coordinator").Logger(),
		states:  map[string]string{},
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.coordinator.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	incidentID := worker.ResolveIncidentID(event)
	if incidentID == "" {
		return nil
	}
	var target string
	switch event.Envelope.EventType {
	case "incident.opened":
		target = eventutil.StateDiagnosing
	case "diagnosis.created", "recommended_actions.created":
		target = eventutil.StateMitigating
	case "action.applied":
		target = eventutil.StateMonitoring
	case "action.failed":
		target = eventutil.StateEscalated
	case "action.rolled_back":
		target = eventutil.StateOpen
	default:
		return nil
	}

	w.mu.Lock()
	current := w.states[incidentID]
	if current == "" {
		current = eventutil.StateOpen
	}
	if current == target {
		w.mu.Unlock()
		return nil
	}
	w.states[incidentID] = target
	w.mu.Unlock()

	err := w.emitStateChange(ctx, event.Envelope.OrgID, incidentID, current, target, "coordinator_transition")

	if target == eventutil.StateResolved || target == eventutil.StateEscalated {
		w.mu.Lock()
		delete(w.states, incidentID)
		w.mu.Unlock()
	}

	return err
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
