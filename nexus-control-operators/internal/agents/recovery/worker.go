package recovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	recoveryworker "nexus-control-operators/internal/agents/recovery/worker"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Config struct {
	RequiredSuccesses int
	MonitoringWindow  time.Duration
	Now               func() time.Time
}

type mitigationTrack struct {
	OrgID              uuid.UUID
	IncidentID         string
	ActionID           string
	ActionType         string
	SuccessCount       int
	MonitoringDeadline time.Time
	TTLDeadline        *time.Time
}

type Worker struct {
	emitter           EventEmitter
	requiredSuccesses int
	monitoringWindow  time.Duration
	now               func() time.Time
	mu                sync.Mutex
	tracks            map[string]mitigationTrack
}

func NewWorker(emitter EventEmitter, cfg Config) *Worker {
	required := cfg.RequiredSuccesses
	if required <= 0 {
		required = 3
	}
	monitoringWindow := cfg.MonitoringWindow
	if monitoringWindow <= 0 {
		monitoringWindow = 5 * time.Minute
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &Worker{
		emitter:           emitter,
		requiredSuccesses: required,
		monitoringWindow:  monitoringWindow,
		now:               nowFn,
		tracks:            map[string]mitigationTrack{},
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.recovery.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	incidentID := recoveryworker.ResolveIncidentID(event)
	if incidentID == "" {
		return nil
	}
	eventTime := w.eventTime(event)

	// Evaluate time-based states first for every incoming event.
	if err := w.evaluateIncident(ctx, incidentID, event.Envelope.OrgID, eventTime); err != nil {
		return err
	}

	switch event.Envelope.EventType {
	case "action.applied":
		track := mitigationTrack{
			OrgID:              event.Envelope.OrgID,
			IncidentID:         incidentID,
			ActionID:           strings.TrimSpace(recoveryworker.AsString(event.Envelope.Payload["action_id"])),
			ActionType:         strings.TrimSpace(recoveryworker.AsString(event.Envelope.Payload["action_type"])),
			SuccessCount:       0,
			MonitoringDeadline: eventTime.Add(w.monitoringWindow),
		}
		ttlSeconds := recoveryworker.AsInt(event.Envelope.Payload["ttl_seconds"], 0)
		if ttlSeconds > 0 {
			deadline := eventTime.Add(time.Duration(ttlSeconds) * time.Second)
			track.TTLDeadline = &deadline
		}
		w.mu.Lock()
		w.tracks[incidentID] = track
		w.mu.Unlock()
		return w.emitState(ctx, event.Envelope.OrgID, incidentID, "MITIGATING", "MONITORING", "post_apply_monitoring")
	case "tool_call.finished":
		status := strings.ToLower(strings.TrimSpace(recoveryworker.AsString(event.Envelope.Payload["status"])))
		w.mu.Lock()
		track, tracked := w.tracks[incidentID]
		w.mu.Unlock()
		if !tracked {
			return nil
		}
		if status == "success" {
			w.mu.Lock()
			track.SuccessCount++
			w.tracks[incidentID] = track
			w.mu.Unlock()
			return w.evaluateIncident(ctx, incidentID, event.Envelope.OrgID, eventTime)
		}
		w.mu.Lock()
		delete(w.tracks, incidentID)
		w.mu.Unlock()
		if err := w.emitActionRollback(ctx, event.Envelope.OrgID, incidentID, track.ActionID, track.ActionType, "post_mitigation_regression"); err != nil {
			return err
		}
		return w.emitState(ctx, event.Envelope.OrgID, incidentID, "MONITORING", "OPEN", "regressed_after_mitigation")
	default:
		return nil
	}
}

func (w *Worker) OnIdle(ctx context.Context) error {
	now := w.now()
	w.mu.Lock()
	keys := make([]string, 0, len(w.tracks))
	for incidentID := range w.tracks {
		keys = append(keys, incidentID)
	}
	w.mu.Unlock()
	for _, incidentID := range keys {
		w.mu.Lock()
		track, ok := w.tracks[incidentID]
		w.mu.Unlock()
		if !ok {
			continue
		}
		if err := w.evaluateIncident(ctx, incidentID, track.OrgID, now); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) IdleInterval() time.Duration {
	return 1 * time.Second
}

func (w *Worker) evaluateIncident(ctx context.Context, incidentID string, orgID uuid.UUID, at time.Time) error {
	w.mu.Lock()
	track, ok := w.tracks[incidentID]
	w.mu.Unlock()
	if !ok {
		return nil
	}

	if track.TTLDeadline != nil && !at.Before(*track.TTLDeadline) {
		w.mu.Lock()
		delete(w.tracks, incidentID)
		w.mu.Unlock()
		if err := w.emitActionRollback(ctx, orgID, incidentID, track.ActionID, track.ActionType, "ttl_expired"); err != nil {
			return err
		}
		return w.emitState(ctx, orgID, incidentID, "MONITORING", "OPEN", "ttl_expired_auto_rollback")
	}

	if !at.Before(track.MonitoringDeadline) && track.SuccessCount >= w.requiredSuccesses {
		w.mu.Lock()
		delete(w.tracks, incidentID)
		w.mu.Unlock()
		return w.emitState(ctx, orgID, incidentID, "MONITORING", "RESOLVED", "stable_after_mitigation_window")
	}

	return nil
}

func (w *Worker) eventTime(event opsdomain.StoredEvent) time.Time {
	if !event.Envelope.OccurredAt.IsZero() {
		return event.Envelope.OccurredAt.UTC()
	}
	if !event.CreatedAt.IsZero() {
		return event.CreatedAt.UTC()
	}
	return w.now()
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

func (w *Worker) emitActionRollback(ctx context.Context, orgID uuid.UUID, incidentID, actionID, actionType, reason string) error {
	if w.emitter == nil {
		return nil
	}
	incID := incidentID
	actorID := "agents.recovery"
	if strings.TrimSpace(reason) == "" {
		reason = "manual_or_automatic_rollback"
	}
	payload := map[string]any{
		"incident_id": incidentID,
		"reason":      reason,
	}
	if strings.TrimSpace(actionID) != "" {
		payload["action_id"] = actionID
	}
	if strings.TrimSpace(actionType) != "" {
		payload["action_type"] = actionType
	}
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
		Source:  "agents.recovery",
		Payload: payload,
	})
	return err
}

