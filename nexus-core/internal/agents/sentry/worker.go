package sentry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"nexus-core/internal/incidents"
	incidentdomain "nexus-core/internal/incidents/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type Baseline struct {
	OrgID       uuid.UUID
	ToolName    string
	Metric      string
	EWMA        float64
	SampleCount int64
}

type FingerprintState struct {
	OrgID       uuid.UUID
	Fingerprint string
	IncidentID  *uuid.UUID
	State       string
}

type StateRepository interface {
	GetBaseline(ctx context.Context, orgID uuid.UUID, toolName, metric string) (Baseline, error)
	UpsertBaseline(ctx context.Context, b Baseline) error
	GetFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (FingerprintState, error)
	UpsertFingerprint(ctx context.Context, f FingerprintState) error
}

type IncidentPort interface {
	Create(ctx context.Context, orgID uuid.UUID, actor *string, req incidents.CreateRequest) (incidentdomain.Incident, error)
}

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Config struct {
	Alpha            float64
	ErrorThreshold   float64
	MinSamples       int64
	SignalToolMetric string
}

type Worker struct {
	state     StateRepository
	incidents IncidentPort
	emitter   EventEmitter
	cfg       Config
}

func NewWorker(state StateRepository, incidents IncidentPort, emitter EventEmitter, cfg Config) *Worker {
	if cfg.Alpha <= 0 || cfg.Alpha > 1 {
		cfg.Alpha = 0.30
	}
	if cfg.ErrorThreshold <= 0 || cfg.ErrorThreshold >= 1 {
		cfg.ErrorThreshold = 0.35
	}
	if cfg.MinSamples <= 0 {
		cfg.MinSamples = 20
	}
	if strings.TrimSpace(cfg.SignalToolMetric) == "" {
		cfg.SignalToolMetric = "error_rate"
	}
	return &Worker{
		state:     state,
		incidents: incidents,
		emitter:   emitter,
		cfg:       cfg,
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.sentry.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	switch event.Envelope.EventType {
	case "tool_call.finished", "policy.denied", "quota.exceeded", "tool_degraded":
	default:
		return nil
	}
	if event.Envelope.EventType != "tool_call.finished" {
		return nil
	}

	orgID := event.Envelope.OrgID
	toolName := strings.TrimSpace(asString(event.Envelope.Payload["tool_name"]))
	if toolName == "" {
		return nil
	}
	status := strings.ToLower(strings.TrimSpace(asString(event.Envelope.Payload["status"])))
	sample := 0.0
	if status != "success" {
		sample = 1
	}

	baseline, err := w.state.GetBaseline(ctx, orgID, toolName, w.cfg.SignalToolMetric)
	if err != nil {
		return err
	}
	if baseline.SampleCount <= 0 {
		baseline.EWMA = sample
	} else {
		baseline.EWMA = w.cfg.Alpha*sample + (1-w.cfg.Alpha)*baseline.EWMA
	}
	baseline.OrgID = orgID
	baseline.ToolName = toolName
	baseline.Metric = w.cfg.SignalToolMetric
	baseline.SampleCount++
	if err := w.state.UpsertBaseline(ctx, baseline); err != nil {
		return err
	}

	if baseline.SampleCount < w.cfg.MinSamples || baseline.EWMA < w.cfg.ErrorThreshold || sample == 0 {
		return nil
	}

	fingerprint := fmt.Sprintf("fp:%s:%s:%s", orgID.String(), toolName, w.cfg.SignalToolMetric)
	_ = w.emit(ctx, orgID, "anomaly.detected", map[string]any{
		"fingerprint":      fingerprint,
		"signal":           "error_rate_spike",
		"tool_name":        toolName,
		"observed_value":   baseline.EWMA,
		"threshold_value":  w.cfg.ErrorThreshold,
		"window_size":      baseline.SampleCount,
		"evidence_refs":    []string{"event_store:" + fmt.Sprintf("%d", event.Sequence)},
	})

	fpState, err := w.state.GetFingerprint(ctx, orgID, fingerprint)
	if err != nil {
		return err
	}
	if fpState.IncidentID != nil && strings.EqualFold(fpState.State, "open") {
		return nil
	}
	if w.incidents == nil {
		return nil
	}

	actorID := "agents.sentry"
	inc, err := w.incidents.Create(ctx, orgID, &actorID, incidents.CreateRequest{
		Severity: "HIGH",
		Title:    "Anomaly detected by sentry",
		Summary:  "Error rate spike detected for " + toolName,
		EvidenceRefs: []string{
			"event_store:" + fmt.Sprintf("%d", event.Sequence),
			"fingerprint:" + fingerprint,
		},
	})
	if err != nil {
		return err
	}
	fpState = FingerprintState{
		OrgID:       orgID,
		Fingerprint: fingerprint,
		IncidentID:  &inc.ID,
		State:       "open",
	}
	if err := w.state.UpsertFingerprint(ctx, fpState); err != nil {
		return err
	}
	_ = w.emit(ctx, orgID, "incident.opened", map[string]any{
		"incident_id":  inc.ID.String(),
		"severity":     string(inc.Severity),
		"state":        "OPEN",
		"title":        inc.Title,
		"summary":      inc.Summary,
		"fingerprint":  fingerprint,
		"opened_at":    time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (w *Worker) emit(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error {
	if w.emitter == nil {
		return nil
	}
	actorID := "agents.sentry"
	_, err := w.emitter.Emit(ctx, opseventstore.EmitInput{
		EventType: eventType,
		Version:   1,
		OrgID:     orgID,
		Actor: opsdomain.Actor{
			ActorID:   &actorID,
			ActorType: "agent",
		},
		Source:  "agents.sentry",
		Payload: payload,
	})
	return err
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
