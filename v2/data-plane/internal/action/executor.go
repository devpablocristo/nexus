package action

import (
	"context"
	"encoding/json"
	"fmt"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type Executor interface {
	Execute(ctx context.Context, action actiondomain.Action, executedBy actiondomain.ActorRef) (map[string]any, error)
}

type DeterministicExecutor struct{}

func NewDeterministicExecutor() *DeterministicExecutor {
	return &DeterministicExecutor{}
}

func (e *DeterministicExecutor) Execute(ctx context.Context, action actiondomain.Action, executedBy actiondomain.ActorRef) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var payload any
	if len(action.Payload) > 0 {
		if err := json.Unmarshal(action.Payload, &payload); err != nil {
			return nil, fmt.Errorf("decode action payload: %w", err)
		}
	}

	return map[string]any{
		"mode":         "deterministic_simulation",
		"execution_id": "exec_" + action.ID.String(),
		"action_id":    action.ID.String(),
		"action_type":  string(action.Type),
		"resource_id":  action.ResourceID,
		"executed_by": map[string]any{
			"type": string(executedBy.Type),
			"id":   executedBy.ID,
		},
		"payload": payload,
	}, nil
}
