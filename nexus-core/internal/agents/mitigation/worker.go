package mitigation

import (
	"context"
	"strings"

	"github.com/google/uuid"
	opsaction "nexus-core/internal/ops/actionengine"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type Worker struct {
	engine opsaction.Engine
}

func NewWorker(engine opsaction.Engine) *Worker {
	return &Worker{engine: engine}
}

func (w *Worker) ConsumerGroup() string {
	return "agents.mitigation.v1"
}

func (w *Worker) Handle(ctx context.Context, event opsdomain.StoredEvent) error {
	if event.Envelope.EventType != "recommended_actions.created" {
		return nil
	}
	if w.engine == nil {
		return nil
	}
	incidentID := resolveIncidentID(event)
	actions := toAnySlice(event.Envelope.Payload["actions"])
	for _, actionAny := range actions {
		actionMap, ok := actionAny.(map[string]any)
		if !ok {
			continue
		}
		req := opsaction.EngineRequest{
			IncidentID:   incidentID,
			ActionType:   strings.TrimSpace(asString(actionMap["action_type"])),
			Scope:        toMap(actionMap["scope"]),
			TTLSeconds:   asInt(actionMap["ttl_seconds"], 600),
			Params:       toMap(actionMap["params"]),
			EvidenceRefs: toStringSlice(actionMap["evidence_refs"]),
		}
		dryRun, err := w.engine.DryRun(ctx, event.Envelope.OrgID, ptr("agents.mitigation"), req)
		if err != nil {
			return err
		}
		if dryRun.ApprovalRequired {
			continue
		}
		proposalID := dryRun.Proposal.ID
		if _, err := w.engine.Apply(ctx, event.Envelope.OrgID, ptr("agents.mitigation"), opsaction.EngineRequest{
			ProposalID: &proposalID,
		}); err != nil {
			return err
		}
	}
	return nil
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

func toAnySlice(v any) []any {
	if arr, ok := v.([]any); ok {
		return arr
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[string]interface{}); ok {
		out := map[string]any{}
		for k, val := range m {
			out[k] = val
		}
		return out
	}
	return map[string]any{}
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

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any, def int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return def
	}
}

func ptr(v string) *string {
	return &v
}
