package llm

import (
	"context"
	"strings"
)

type mockProvider struct{}

func NewMockProvider() Provider {
	return &mockProvider{}
}

func (m *mockProvider) Generate(_ context.Context, req ProviderRequest) (map[string]any, error) {
	switch strings.ToLower(strings.TrimSpace(req.Task)) {
	case "diagnosis":
		return map[string]any{
			"summary_30s": "Se observa degradacion concentrada en un tool con saturacion de bucket.",
			"root_cause":  "rate_limit_bucket_saturated",
			"confidence":  0.82,
			"evidence_refs": []any{
				"audit:req-1001",
				"metrics:tool:p95",
			},
			"hypotheses": []any{
				map[string]any{
					"title":      "Rate limit configurado por debajo de la demanda",
					"confidence": 0.82,
					"evidence_refs": []any{
						"metrics:bucket:utilization",
					},
				},
			},
			"recommended_actions": []any{
				map[string]any{
					"action_type": "set_rate_limit",
					"scope": map[string]any{
						"level":   "tool",
						"org_id":  asString(req.Input["org_id"]),
						"tool_id": "echo",
					},
					"ttl_seconds": 600,
					"params": map[string]any{
						"rpm":     180,
						"tool_id": "echo",
					},
					"evidence_refs": []any{
						"audit:req-1001",
					},
				},
			},
			"missing_info": []any{},
		}, nil
	case "communication_plan":
		return map[string]any{
			"incident_id": asString(req.Input["incident_id"]),
			"summary":     "Incidente detectado, mitigacion temporal aplicada y monitoreo en curso.",
			"root_cause_claim": map[string]any{
				"value":      "rate_limit_bucket_saturated",
				"confidence": 0.8,
			},
			"evidence_refs": []any{
				"audit:req-1001",
			},
			"audiences": []any{
				map[string]any{"name": "oncall", "channel": "internal"},
			},
			"drafts": []any{
				map[string]any{
					"audience":          "oncall",
					"subject":           "Incident update",
					"body":              "Mitigacion aplicada con TTL temporal.",
					"approval_required": false,
				},
			},
		}, nil
	case "executive_qa":
		return map[string]any{
			"answer": "El incidente esta mitigado temporalmente con cambio de rate limit.",
			"evidence_refs": []any{
				"incident:latest",
				"action:last_applied",
			},
			"recommended_action": map[string]any{
				"action_type": "set_rate_limit",
				"scope": map[string]any{
					"level":   "tool",
					"org_id":  asString(req.Input["org_id"]),
					"tool_id": "echo",
				},
				"ttl_seconds": 600,
				"params": map[string]any{
					"rpm":     200,
					"tool_id": "echo",
				},
				"evidence_refs": []any{
					"incident:latest",
				},
			},
		}, nil
	default:
		return map[string]any{
			"root_cause":  "unknown",
			"confidence":  0.0,
			"missing_info": []any{"task_not_supported"},
		}, nil
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
