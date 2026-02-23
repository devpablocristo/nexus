package executive_qa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	actiondomain "nexus-core/internal/ops/actionengine/usecases/domain"
	"nexus-core/internal/ops/actionengine"
	"nexus-core/internal/ops/llm"
)

func TestService_AskCreatesActionProposalViaEngine(t *testing.T) {
	t.Parallel()
	orgID := uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54")
	svc := NewService(llmQAStub{}, actionEngineStub{})

	out, err := svc.Ask(context.Background(), orgID, ptr("alice"), AskRequest{
		Question: "How do we stabilize this incident?",
	})
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if out.ProposedActionID == nil || *out.ProposedActionID == "" {
		t.Fatalf("expected proposed action id")
	}
}

type llmQAStub struct{}

func (llmQAStub) Generate(context.Context, llm.Request) (map[string]any, error) {
	return map[string]any{
		"answer": "Increase rate limit temporarily.",
		"evidence_refs": []any{
			"incident:latest",
		},
		"recommended_action": map[string]any{
			"action_type": "set_rate_limit",
			"scope": map[string]any{
				"level":   "tool",
				"org_id":  "996e9e43-7bab-4e68-a831-0a766befbf54",
				"tool_id": "world.move",
			},
			"ttl_seconds": 600,
			"params": map[string]any{
				"rpm":     200,
				"tool_id": "world.move",
			},
			"evidence_refs": []any{
				"incident:latest",
			},
		},
	}, nil
}

func (llmQAStub) GenerateStrict(context.Context, llm.Request, string) (map[string]any, error) {
	return nil, nil
}

type actionEngineStub struct{}

func (actionEngineStub) DryRun(context.Context, uuid.UUID, *string, actionengine.EngineRequest) (actionengine.EngineResult, error) {
	return actionengine.EngineResult{
		Proposal: actiondomain.Proposal{
			ID:         uuid.New(),
			ActionType: "set_rate_limit",
		},
	}, nil
}

func (actionEngineStub) Apply(context.Context, uuid.UUID, *string, actionengine.EngineRequest) (actionengine.EngineResult, error) {
	return actionengine.EngineResult{}, nil
}

func (actionEngineStub) Rollback(context.Context, uuid.UUID, *string, actionengine.EngineRequest) (actionengine.EngineResult, error) {
	return actionengine.EngineResult{}, nil
}

func ptr(v string) *string {
	return &v
}
