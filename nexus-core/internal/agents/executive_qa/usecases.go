package executive_qa

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	opsaction "nexus-core/internal/ops/actionengine"
	"nexus-core/internal/ops/llm"
	"nexus-core/pkg/types"
)

type AskRequest struct {
	Question   string
	IncidentID *uuid.UUID
}

type AskResponse struct {
	Answer             string
	EvidenceRefs       []string
	ProposedActionID   *string
	ProposedActionType *string
}

type Usecases struct {
	llmClient    llm.Client
	actionEngine opsaction.Engine
}

func NewUsecases(llmClient llm.Client, actionEngine opsaction.Engine) *Usecases {
	return &Usecases{
		llmClient:    llmClient,
		actionEngine: actionEngine,
	}
}

func (u *Usecases) Ask(ctx context.Context, orgID uuid.UUID, actor *string, req AskRequest) (AskResponse, error) {
	if strings.TrimSpace(req.Question) == "" {
		return AskResponse{}, types.NewHTTPError(400, types.ErrCodeValidation, "question is required")
	}
	raw, err := u.llmClient.GenerateStrict(ctx, llm.Request{
		Task: "executive_qa",
		Input: map[string]any{
			"org_id":      orgID.String(),
			"incident_id": uuidToString(req.IncidentID),
			"question":    req.Question,
		},
	}, "executive_qa_response.json")
	if err != nil {
		return AskResponse{
			Answer:       "unknown",
			EvidenceRefs: []string{fmt.Sprintf("llm_validation_error:%s", sanitizeReason(err.Error()))},
		}, nil
	}

	resp := AskResponse{
		Answer:       strings.TrimSpace(asString(raw["answer"])),
		EvidenceRefs: toStringSlice(raw["evidence_refs"]),
	}
	if rec, ok := raw["recommended_action"].(map[string]any); ok && u.actionEngine != nil {
		actionType := strings.TrimSpace(asString(rec["action_type"]))
		if actionType != "" {
			dryRun, dryErr := u.actionEngine.DryRun(ctx, orgID, actor, opsaction.EngineRequest{
				IncidentID:   req.IncidentID,
				ActionType:   actionType,
				Scope:        toMap(rec["scope"]),
				TTLSeconds:   asInt(rec["ttl_seconds"], 600),
				Params:       toMap(rec["params"]),
				EvidenceRefs: toStringSlice(rec["evidence_refs"]),
			})
			if dryErr == nil {
				id := dryRun.Proposal.ID.String()
				resp.ProposedActionID = &id
				resp.ProposedActionType = &dryRun.Proposal.ActionType
			}
		}
	}
	return resp, nil
}

func sanitizeReason(msg string) string {
	clean := strings.TrimSpace(msg)
	clean = strings.ReplaceAll(clean, "\n", " ")
	if len(clean) > 120 {
		clean = clean[:120]
	}
	return clean
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func toStringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if arr, ok2 := v.([]interface{}); ok2 {
			raw = arr
		}
	}
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
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

func uuidToString(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}
