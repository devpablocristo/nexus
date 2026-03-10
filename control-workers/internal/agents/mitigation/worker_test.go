package mitigation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"control-workers/internal/ops/actionengine"
	actiondomain "control-workers/internal/ops/actionengine/usecases/domain"
	opsdomain "control-workers/internal/ops/eventstore/usecases/domain"
)

func TestMitigationWorker_AppliesOnlyNonApprovalActions(t *testing.T) {
	t.Parallel()
	engine := &engineStub{}
	w := NewWorker(engine, zerolog.Nop())

	incidentID := "f503f46f-c137-4165-b9ca-999d0d6f328f"
	err := w.Handle(context.Background(), opsdomain.StoredEvent{
		Envelope: opsdomain.Envelope{
			OrgID:     uuid.MustParse("996e9e43-7bab-4e68-a831-0a766befbf54"),
			EventType: "recommended_actions.created",
			Correlation: opsdomain.Correlation{
				IncidentID: &incidentID,
			},
			Payload: map[string]any{
				"actions": []any{
					map[string]any{
						"action_type": "set_rate_limit",
						"scope": map[string]any{
							"level":   "tool",
							"org_id":  "996e9e43-7bab-4e68-a831-0a766befbf54",
							"tool_id": "echo",
						},
						"ttl_seconds": 600,
						"params": map[string]any{
							"rpm":     120,
							"tool_id": "echo",
						},
						"lease_headers": map[string]any{
							"X-Nexus-Execution-Token": "lease-token",
							"X-Nexus-Lease-Id":        "lease-1",
							"X-Nexus-Credential-Mode": "aws_sts",
						},
					},
					map[string]any{
						"action_type": "quarantine_tenant",
						"scope": map[string]any{
							"level":  "org",
							"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
						},
						"ttl_seconds": 300,
						"params": map[string]any{
							"org_id": "996e9e43-7bab-4e68-a831-0a766befbf54",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if engine.dryRunCalls != 2 {
		t.Fatalf("expected 2 dry-run calls, got=%d", engine.dryRunCalls)
	}
	if engine.applyCalls != 1 {
		t.Fatalf("expected 1 apply call, got=%d", engine.applyCalls)
	}
	if got := engine.applyReq.LeaseHeaders["X-Nexus-Execution-Token"]; got != "lease-token" {
		t.Fatalf("expected lease token to propagate, got=%q", got)
	}
}

type engineStub struct {
	dryRunCalls int
	applyCalls  int
	applyReq    actionengine.EngineRequest
}

func (e *engineStub) DryRun(context.Context, uuid.UUID, *string, actionengine.EngineRequest) (actionengine.EngineResult, error) {
	e.dryRunCalls++
	approvalRequired := e.dryRunCalls == 2
	return actionengine.EngineResult{
		Proposal: actiondomain.Proposal{
			ID: uuid.New(),
		},
		ApprovalRequired: approvalRequired,
	}, nil
}

func (e *engineStub) Apply(_ context.Context, _ uuid.UUID, _ *string, req actionengine.EngineRequest) (actionengine.EngineResult, error) {
	e.applyCalls++
	e.applyReq = req
	return actionengine.EngineResult{}, nil
}

func (e *engineStub) Rollback(context.Context, uuid.UUID, *string, actionengine.EngineRequest) (actionengine.EngineResult, error) {
	return actionengine.EngineResult{}, nil
}
