package sentry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-control-operators/internal/agents/sentry/worker"
	"nexus-control-operators/internal/incidents"
	incidentdomain "nexus-control-operators/internal/incidents/usecases/domain"
	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
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
	P95LatencyMS     float64
}

type Worker struct {
	state     StateRepository
	incidents IncidentPort
	emitter   EventEmitter
	cfg       Config
	log       zerolog.Logger
}

func NewWorker(state StateRepository, incidents IncidentPort, emitter EventEmitter, cfg Config, log zerolog.Logger) *Worker {
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
	if cfg.P95LatencyMS <= 0 {
		cfg.P95LatencyMS = 2000
	}
	return &Worker{
		state:     state,
		incidents: incidents,
		emitter:   emitter,
		cfg:       cfg,
		log:       log.With().Str("worker", "sentry").Logger(),
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

	toolName := worker.ResolveToolName(event.Envelope.Payload)
	if toolName == "" {
		toolName = "unknown_tool"
	}
	metric, signal, sample, forceAnomaly, observedOverride, thresholdOverride := w.measurementFor(event)
	if metric == "" {
		return nil
	}

	orgID := event.Envelope.OrgID
	baseline, err := w.state.GetBaseline(ctx, orgID, toolName, metric)
	if err != nil {
		return err
	}
	baseline, err = w.updateBaseline(ctx, orgID, toolName, metric, sample, baseline)
	if err != nil {
		return err
	}

	threshold := w.cfg.ErrorThreshold
	if thresholdOverride > 0 {
		threshold = thresholdOverride
	}
	observed := baseline.EWMA
	if observedOverride > 0 {
		observed = observedOverride
	}
	if !w.isAnomaly(event, baseline, sample, forceAnomaly, observedOverride, thresholdOverride) {
		return nil
	}

	fingerprint := fmt.Sprintf("fp:%s:%s:%s", orgID.String(), toolName, signal)
	w.emitOrLog(ctx, orgID, "anomaly.detected", map[string]any{
		"fingerprint":     fingerprint,
		"signal":          signal,
		"tool_name":       toolName,
		"observed_value":  worker.RoundFloat(observed, 4),
		"threshold_value": worker.RoundFloat(threshold, 4),
		"window_size":     baseline.SampleCount,
		"evidence_refs":   []string{"event_store:" + fmt.Sprintf("%d", event.Sequence)},
	})

	fpState, err := w.state.GetFingerprint(ctx, orgID, fingerprint)
	if err != nil {
		return err
	}
	if fpState.IncidentID != nil && strings.EqualFold(fpState.State, "open") {
		return nil
	}
	return w.createIncidentAndEmit(ctx, orgID, fingerprint, event)
}

func (w *Worker) measurementFor(event opsdomain.StoredEvent) (metric, signal string, sample float64, forceAnomaly bool, observedOverride, thresholdOverride float64) {
	switch event.Envelope.EventType {
	case "tool_call.finished":
		status := strings.ToLower(strings.TrimSpace(worker.AsString(event.Envelope.Payload["status"])))
		sample = 0
		if status != "success" {
			sample = 1
		}
		return w.cfg.SignalToolMetric, "error_rate_spike", sample, false, 0, 0
	case "policy.denied":
		return "policy_denied_rate", "policy_denied_spike", 1, false, 0, 0
	case "quota.exceeded":
		return "quota_exceeded_rate", "quota_exceeded_spike", 1, false, 0, 0
	case "tool_degraded":
		degradationType := strings.ToLower(strings.TrimSpace(worker.AsString(event.Envelope.Payload["degradation_type"])))
		if strings.Contains(degradationType, "p95") || strings.Contains(degradationType, "latency") {
			p95 := worker.AsFloat(event.Envelope.Payload["p95_latency_ms"])
			if p95 <= 0 {
				p95 = worker.AsFloat(event.Envelope.Payload["p95_ms"])
			}
			if p95 <= 0 {
				p95 = w.cfg.P95LatencyMS
			}
			return "p95_latency_ms", "p95_latency_spike", 1, true, p95, w.cfg.P95LatencyMS
		}
		return "degraded_rate", "tool_degraded", 1, true, 1, 1
	default:
		return "", "", 0, false, 0, 0
	}
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

// updateBaseline actualiza el baseline EWMA, persiste y devuelve el baseline actualizado.
func (w *Worker) updateBaseline(ctx context.Context, orgID uuid.UUID, toolName, metric string, sample float64, baseline Baseline) (Baseline, error) {
	if baseline.SampleCount <= 0 {
		baseline.EWMA = sample
	} else {
		baseline.EWMA = w.cfg.Alpha*sample + (1-w.cfg.Alpha)*baseline.EWMA
	}
	baseline.OrgID = orgID
	baseline.ToolName = toolName
	baseline.Metric = metric
	baseline.SampleCount++
	err := w.state.UpsertBaseline(ctx, baseline)
	return baseline, err
}

// isAnomaly determina si el sample debe considerarse anomalía.
func (w *Worker) isAnomaly(event opsdomain.StoredEvent, baseline Baseline, sample float64, forceAnomaly bool, observedOverride, thresholdOverride float64) bool {
	if sample == 0 {
		return false
	}
	threshold := w.cfg.ErrorThreshold
	if thresholdOverride > 0 {
		threshold = thresholdOverride
	}
	minSamples := w.cfg.MinSamples
	if event.Envelope.EventType != "tool_call.finished" {
		minSamples = 1
	}
	return forceAnomaly || (baseline.SampleCount >= minSamples && baseline.EWMA >= threshold)
}

// createIncidentAndEmit crea el incidente, persiste fingerprint y emite evento.
func (w *Worker) createIncidentAndEmit(ctx context.Context, orgID uuid.UUID, fingerprint string, event opsdomain.StoredEvent) error {
	if w.incidents == nil {
		return nil
	}
	actorID := "agents.sentry"
	inc, err := w.incidents.Create(ctx, orgID, &actorID, incidents.CreateRequest{
		Severity: "HIGH",
		Title:    "Anomaly detected by sentry",
		Summary:  "Error rate spike detected for " + worker.ResolveToolName(event.Envelope.Payload),
		EvidenceRefs: []string{
			"event_store:" + fmt.Sprintf("%d", event.Sequence),
			"fingerprint:" + fingerprint,
		},
	})
	if err != nil {
		return err
	}
	fpState := FingerprintState{
		OrgID:       orgID,
		Fingerprint: fingerprint,
		IncidentID:  &inc.ID,
		State:       "open",
	}
	if err := w.state.UpsertFingerprint(ctx, fpState); err != nil {
		return err
	}
	w.emitOrLog(ctx, orgID, "incident.opened", map[string]any{
		"incident_id": inc.ID.String(),
		"severity":    string(inc.Severity),
		"state":       "OPEN",
		"title":       inc.Title,
		"summary":     inc.Summary,
		"fingerprint": fingerprint,
		"opened_at":   time.Now().UTC().Format(time.RFC3339),
	})
	return nil
}

func (w *Worker) emitOrLog(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) {
	if err := w.emit(ctx, orgID, eventType, payload); err != nil {
		w.log.Warn().Err(err).Str("event_type", eventType).Msg("failed to emit event")
	}
}
