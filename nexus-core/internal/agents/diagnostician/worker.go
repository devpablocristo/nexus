package diagnostician

import (
	"context"
	"strings"

	"github.com/google/uuid"
	diagnosisdomain "nexus-core/internal/ops/diagnosis/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	"nexus-core/internal/ops/llm"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type diagnosisPort interface {
	Create(ctx context.Context, in diagnosisdomain.Report) (diagnosisdomain.Report, error)
	ListByIncident(ctx context.Context, orgID uuid.UUID, incidentID uuid.UUID, limit int) ([]diagnosisdomain.Report, error)
}

type Worker struct {
	llmClient  llm.Client
	diagnosis  diagnosisPort
	emitter    EventEmitter
	provider   string
	model      string
}

func NewWorker(llmClient llm.Client, diagnosis diagnosisPort, emitter EventEmitter, provider, model string) *Worker {
	return &Worker{
		llmClient: llmClient,
		diagnosis: diagnosis,
		emitter:   emitter,
		provider:  provider,
		model:     model,
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.diagnostician.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	if event.Envelope.EventType != "incident.opened" {
		return nil
	}
	incidentID := resolveIncidentID(event)
	if incidentID == nil {
		return nil
	}

	input := map[string]any{
		"org_id":      event.Envelope.OrgID.String(),
		"incident_id": incidentID.String(),
		"event_payload": event.Envelope.Payload,
	}
	report, err := w.llmClient.GenerateStrict(ctx, llm.Request{
		Task:  "diagnosis",
		Input: input,
	}, "diagnosis_report.json")

	status := diagnosisdomain.StatusValid
	var validationErr *string
	if err != nil {
		status = diagnosisdomain.StatusInvalid
		msg := err.Error()
		validationErr = &msg
		report = map[string]any{
			"summary_30s":         "Insufficient evidence to determine root cause.",
			"root_cause":          "unknown",
			"confidence":          0.0,
			"hypotheses":          []any{},
			"recommended_actions": []any{},
			"missing_info":        []any{"llm_output_invalid_or_missing_evidence"},
		}
	}

	created, createErr := w.diagnosis.Create(ctx, diagnosisdomain.Report{
		OrgID:           event.Envelope.OrgID,
		IncidentID:      incidentID,
		Provider:        w.provider,
		Model:           w.model,
		Status:          status,
		Report:          report,
		ValidationError: validationErr,
		CreatedBy:       ptr("agents.diagnostician"),
	})
	if createErr != nil {
		return createErr
	}

	rootCause := strings.TrimSpace(asString(report["root_cause"]))
	confidence := asFloat(report["confidence"])
	payload := map[string]any{
		"incident_id":  incidentID.String(),
		"diagnosis_id": created.ID.String(),
		"root_cause":   rootCause,
		"confidence":   confidence,
	}
	if refs := toStringSlice(report["evidence_refs"]); len(refs) > 0 {
		payload["evidence_refs"] = refs
	}
	_ = w.emit(ctx, event.Envelope.OrgID, incidentID, "diagnosis.created", payload)

	recommended := toAnySlice(report["recommended_actions"])
	if len(recommended) > 0 {
		_ = w.emit(ctx, event.Envelope.OrgID, incidentID, "recommended_actions.created", map[string]any{
			"incident_id":  incidentID.String(),
			"diagnosis_id": created.ID.String(),
			"actions":      recommended,
		})
	}
	return nil
}

func (w *Worker) emit(ctx context.Context, orgID uuid.UUID, incidentID *uuid.UUID, eventType string, payload map[string]any) error {
	if w.emitter == nil {
		return nil
	}
	incID := incidentID.String()
	actorID := "agents.diagnostician"
	_, err := w.emitter.Emit(ctx, opseventstore.EmitInput{
		EventType: eventType,
		Version:   1,
		OrgID:     orgID,
		Correlation: opsdomain.Correlation{
			IncidentID: &incID,
		},
		Actor: opsdomain.Actor{
			ActorID:   &actorID,
			ActorType: "agent",
		},
		Source:  "agents.diagnostician",
		Payload: payload,
	})
	return err
}

func resolveIncidentID(event opsdomain.StoredEvent) *uuid.UUID {
	if event.Envelope.Correlation.IncidentID != nil {
		if id, err := uuid.Parse(*event.Envelope.Correlation.IncidentID); err == nil {
			return &id
		}
	}
	if raw := strings.TrimSpace(asString(event.Envelope.Payload["incident_id"])); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			return &id
		}
	}
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
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

func toStringSlice(v any) []string {
	raw := toAnySlice(v)
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func toAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

func ptr(v string) *string {
	return &v
}
