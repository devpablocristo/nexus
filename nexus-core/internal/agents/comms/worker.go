package comms

import (
	"context"
	"strings"

	"github.com/google/uuid"
	commsmod "nexus-core/internal/ops/comms"
	commsdomain "nexus-core/internal/ops/comms/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	"nexus-core/internal/ops/llm"
)

type EventEmitter interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type Worker struct {
	llmClient llm.Client
	comms     commsmod.Service
	emitter   EventEmitter
}

func NewWorker(llmClient llm.Client, commsSvc commsmod.Service, emitter EventEmitter) *Worker {
	return &Worker{
		llmClient: llmClient,
		comms:     commsSvc,
		emitter:   emitter,
	}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.comms.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	switch event.Envelope.EventType {
	case "incident.opened", "diagnosis.created", "action.applied":
	default:
		return nil
	}
	incidentID := resolveIncidentID(event)
	if incidentID == nil {
		return nil
	}

	plan, err := w.llmClient.GenerateStrict(ctx, llm.Request{
		Task: "communication_plan",
		Input: map[string]any{
			"org_id":      event.Envelope.OrgID.String(),
			"incident_id": incidentID.String(),
		},
	}, "communication_plan.json")
	if err != nil {
		plan = map[string]any{
			"incident_id": incidentID.String(),
			"summary":     "Update interno: incidente en investigación.",
			"root_cause_claim": map[string]any{
				"value":      "unknown",
				"confidence": 0.0,
			},
			"audiences": []any{
				map[string]any{"name": "oncall", "channel": "internal"},
			},
			"drafts": []any{
				map[string]any{
					"audience":          "oncall",
					"subject":           "Incident update",
					"body":              "Investigando incidente. Sin causa raíz confirmada.",
					"approval_required": false,
				},
			},
		}
	}

	drafts := toAnySlice(plan["drafts"])
	if len(drafts) == 0 {
		return nil
	}
	for _, draftAny := range drafts {
		draftMap, ok := draftAny.(map[string]any)
		if !ok {
			continue
		}
		approvalRequired := asBool(draftMap["approval_required"])
		status := commsdomain.StatusSentInternal
		channel := strings.TrimSpace(asString(draftMap["channel"]))
		if channel == "" {
			channel = "internal"
		}
		if approvalRequired {
			status = commsdomain.StatusAwaitingApproval
		}
		created, createErr := w.createDraft(ctx, event.Envelope.OrgID, incidentID, draftMap, plan, channel, approvalRequired, status)
		if createErr != nil {
			return createErr
		}
		w.emitDraftEvents(ctx, event.Envelope.OrgID, incidentID, created, approvalRequired)
	}
	return nil
}

// createDraft crea un draft a partir del plan LLM.
func (w *Worker) createDraft(ctx context.Context, orgID uuid.UUID, incidentID *uuid.UUID, draftMap map[string]any, plan map[string]any, channel string, approvalRequired bool, status commsdomain.Status) (commsdomain.Draft, error) {
	return w.comms.Create(ctx, commsdomain.Draft{
		OrgID:            orgID,
		IncidentID:       incidentID,
		Channel:          channel,
		Audience:         strings.TrimSpace(asString(draftMap["audience"])),
		Status:           status,
		RequiresApproval: approvalRequired,
		Content: map[string]any{
			"subject": asString(draftMap["subject"]),
			"body":    asString(draftMap["body"]),
			"summary": asString(plan["summary"]),
		},
		CreatedBy: ptr("agents.comms"),
	})
}

// emitDraftEvents emite eventos de draft creado / awaiting approval / sent.
func (w *Worker) emitDraftEvents(ctx context.Context, orgID uuid.UUID, incidentID *uuid.UUID, created commsdomain.Draft, approvalRequired bool) {
	w.emitOrLog(ctx, orgID, incidentID, "comms.draft_created", map[string]any{
		"draft_id": created.ID.String(), "incident_id": incidentID.String(),
		"channel": created.Channel, "audience": created.Audience, "content": created.Content["body"],
	})
	if approvalRequired {
		w.emitOrLog(ctx, orgID, incidentID, "comms.awaiting_approval", map[string]any{
			"draft_id": created.ID.String(), "incident_id": incidentID.String(), "approval_scope": "external_comms",
		})
	} else {
		w.emitOrLog(ctx, orgID, incidentID, "comms.sent_internal", map[string]any{
			"draft_id": created.ID.String(), "incident_id": incidentID.String(),
			"channel": "internal", "sent_at": created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

func (w *Worker) emitOrLog(ctx context.Context, orgID uuid.UUID, incidentID *uuid.UUID, eventType string, payload map[string]any) {
	_ = w.emit(ctx, orgID, incidentID, eventType, payload)
}

func (w *Worker) emit(ctx context.Context, orgID uuid.UUID, incidentID *uuid.UUID, eventType string, payload map[string]any) error {
	if w.emitter == nil {
		return nil
	}
	incID := incidentID.String()
	actorID := "agents.comms"
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
		Source:  "agents.comms",
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

func toAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func ptr(v string) *string {
	return &v
}
