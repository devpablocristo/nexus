package sentry

import (
	"context"
	"fmt"
	"math"
	"strings"

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
	P95LatencyMS     float64
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
	if cfg.P95LatencyMS <= 0 {
		cfg.P95LatencyMS = 2000
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

	toolName := resolveToolName(event.Envelope.Payload)
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
		"observed_value":  roundFloat(observed, 4),
		"threshold_value": roundFloat(threshold, 4),
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
		status := strings.ToLower(strings.TrimSpace(asString(event.Envelope.Payload["status"])))
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
		degradationType := strings.ToLower(strings.TrimSpace(asString(event.Envelope.Payload["degradation_type"])))
		if strings.Contains(degradationType, "p95") || strings.Contains(degradationType, "latency") {
			p95 := asFloat(event.Envelope.Payload["p95_latency_ms"])
			if p95 <= 0 {
				p95 = asFloat(event.Envelope.Payload["p95_ms"])
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

func resolveToolName(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if tool := strings.TrimSpace(asString(payload["tool_name"])); tool != "" {
		return tool
	}
	if tool := strings.TrimSpace(asString(payload["tool_id"])); tool != "" {
		return tool
	}
	return ""
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func roundFloat(v float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(v)
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
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
