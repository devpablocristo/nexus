package recovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	recoveryworker "control-workers/internal/agents/recovery/worker"
	opseventstore "control-workers/internal/ops/eventstore"
	opsdomain "control-workers/internal/ops/eventstore/usecases/domain"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Config struct {
	RequiredSuccesses int
	MonitoringWindow  time.Duration
	IdleInterval      time.Duration
	DataDir           string
	Now               func() time.Time
}

type mitigationTrack struct {
	OrgID              uuid.UUID `json:"org_id"`
	IncidentID         string    `json:"incident_id"`
	ActionID           string    `json:"action_id"`
	ActionType         string    `json:"action_type"`
	SuccessCount       int       `json:"success_count"`
	MonitoringDeadline time.Time `json:"monitoring_deadline"`
	TTLDeadline        *time.Time `json:"ttl_deadline,omitempty"`
}

type Worker struct {
	emitter           EventEmitter
	requiredSuccesses int
	monitoringWindow  time.Duration
	idleInterval      time.Duration
	now               func() time.Time
	mu                sync.Mutex
	tracks            map[string]mitigationTrack
	dataDir           string
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
	idleInterval := cfg.IdleInterval
	if idleInterval <= 0 {
		idleInterval = 15 * time.Second
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	w := &Worker{
		emitter:           emitter,
		requiredSuccesses: required,
		monitoringWindow:  monitoringWindow,
		idleInterval:      idleInterval,
		now:               nowFn,
		tracks:            map[string]mitigationTrack{},
		dataDir:           cfg.DataDir,
	}
	w.loadTracks()
	return w
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
		w.persistTracks()
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
			w.persistTracks()
			w.mu.Unlock()
			return w.evaluateIncident(ctx, incidentID, event.Envelope.OrgID, eventTime)
		}
		w.mu.Lock()
		delete(w.tracks, incidentID)
		w.persistTracks()
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
	return w.idleInterval
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
		w.persistTracks()
		w.mu.Unlock()
		if err := w.emitActionRollback(ctx, orgID, incidentID, track.ActionID, track.ActionType, "ttl_expired"); err != nil {
			return err
		}
		return w.emitState(ctx, orgID, incidentID, "MONITORING", "OPEN", "ttl_expired_auto_rollback")
	}

	if !at.Before(track.MonitoringDeadline) && track.SuccessCount >= w.requiredSuccesses {
		w.mu.Lock()
		delete(w.tracks, incidentID)
		w.persistTracks()
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

func (w *Worker) loadTracks() {
	if w.dataDir == "" {
		return
	}
	path := filepath.Join(w.dataDir, "recovery_tracks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var tracks map[string]mitigationTrack
	if json.Unmarshal(data, &tracks) == nil && tracks != nil {
		w.tracks = tracks
	}
}

func (w *Worker) persistTracks() {
	if w.dataDir == "" {
		return
	}
	_ = os.MkdirAll(w.dataDir, 0o755)
	data, err := json.Marshal(w.tracks)
	if err != nil {
		return
	}
	tmp := filepath.Join(w.dataDir, "recovery_tracks.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(w.dataDir, "recovery_tracks.json"))
}
