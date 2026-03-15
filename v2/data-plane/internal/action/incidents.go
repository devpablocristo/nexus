package action

import (
	"context"
	"errors"
	"log"
	"strings"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

type IncidentTrigger string

const (
	IncidentTriggerBlockedAction    IncidentTrigger = "blocked_action"
	IncidentTriggerApprovalRejected IncidentTrigger = "approval_rejected"
	IncidentTriggerExecutionFailed  IncidentTrigger = "execution_failed"
)

type IncidentRequest struct {
	SourceID     string
	ActionType   actiondomain.ActionType
	ResourceID   string
	ResourceType actiondomain.ResourceType
	Trigger      IncidentTrigger
	RiskLevel    actiondomain.RiskLevel
	Summary      string
	Reason       string
	Details      map[string]any
}

type IncidentSink interface {
	Create(ctx context.Context, req IncidentRequest) error
}

func (u *Usecases) WithIncidentSink(sink IncidentSink) *Usecases {
	u.incidents = sink
	return u
}

func (u *Usecases) emitIncident(ctx context.Context, item actiondomain.Action, trigger IncidentTrigger, reason string, details map[string]any) {
	if u.incidents == nil {
		return
	}

	req := IncidentRequest{
		SourceID:     item.ID.String(),
		ActionType:   item.Type,
		ResourceID:   item.ResourceID,
		ResourceType: item.ResourceType,
		Trigger:      trigger,
		RiskLevel:    item.Risk.Level,
		Summary:      incidentSummary(trigger, item.Type),
		Reason:       strings.TrimSpace(reason),
		Details:      cloneMap(details),
	}
	if err := u.incidents.Create(ctx, req); err != nil {
		log.Printf("action incident sink failed: action_id=%s trigger=%s err=%v", item.ID, trigger, err)
	}
}

func incidentSummary(trigger IncidentTrigger, actionType actiondomain.ActionType) string {
	switch trigger {
	case IncidentTriggerBlockedAction:
		return string(actionType) + " blocked by Nexus"
	case IncidentTriggerApprovalRejected:
		return string(actionType) + " rejected during approval"
	case IncidentTriggerExecutionFailed:
		return string(actionType) + " failed during execution"
	default:
		return string(actionType) + " requires operator attention"
	}
}

func blockedIncidentReason(item actiondomain.Action) string {
	for _, evidence := range item.Evidence {
		if evidence.Kind != "policy_decision" {
			continue
		}
		if summary := strings.TrimSpace(evidence.Summary); summary != "" {
			return summary
		}
	}
	return "action blocked by Nexus policy"
}

func rejectionIncidentReason(comment string) string {
	if trimmed := strings.TrimSpace(comment); trimmed != "" {
		return trimmed
	}
	return "action rejected during approval"
}

func executionIncidentReason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "action execution timed out"
	case err != nil && strings.TrimSpace(err.Error()) != "":
		return strings.TrimSpace(err.Error())
	default:
		return "action execution failed"
	}
}

func actorDetails(actor actiondomain.ActorRef) map[string]any {
	return map[string]any{
		"type": string(actor.Type),
		"id":   actor.ID,
	}
}
