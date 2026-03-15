package action

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	actiondomain "nexus/v2/data-plane/internal/action/usecases/domain"
)

var ErrResourceNotFound = fmt.Errorf("resource not found")

func evaluateAction(now time.Time, req CreateRequest, resource actiondomain.ProtectedResource, policies []ActionPolicy, evaluator actionPolicyEvaluator) (actiondomain.Action, error) {
	normalizedPayload, err := normalizePayload(req.ActionType, req.Payload)
	if err != nil {
		return actiondomain.Action{}, err
	}

	risk := riskFor(req.ActionType, resource)
	evidence := buildEvidence(now, normalizedPayload, resource)
	decision := evaluatePolicyDecision(req, resource, policies, evaluator)
	evidence = append(evidence, actiondomain.EvidenceRecord{
		ID:        uuid.New(),
		Kind:      "policy_decision",
		Status:    actiondomain.EvidenceStatusPassed,
		Summary:   decision.summary,
		Details:   map[string]any{"effect": string(decision.effect), "require_approval": decision.requireApproval, "matched_policy_id": decision.policyID},
		CreatedAt: now,
	})

	var approval *actiondomain.Approval
	var status actiondomain.ActionStatus
	var actionDecision actiondomain.Decision

	switch decision.effect {
	case actiondomain.DecisionDeny:
		status = actiondomain.ActionStatusBlocked
		actionDecision = actiondomain.DecisionDeny
	case actiondomain.DecisionRequireApproval:
		status = actiondomain.ActionStatusPendingApproval
		actionDecision = actiondomain.DecisionRequireApproval
		approval = &actiondomain.Approval{
			ID:            uuid.New(),
			Status:        actiondomain.ApprovalStatusPending,
			RequiredCount: 1,
			GrantedCount:  0,
			ExpiresAt:     now.Add(decision.approvalTTL),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
	default:
		status = actiondomain.ActionStatusApproved
		actionDecision = actiondomain.DecisionAllow
	}

	action := actiondomain.Action{
		Type:          req.ActionType,
		Status:        status,
		Decision:      actionDecision,
		ResourceID:    strings.TrimSpace(req.ResourceID),
		ResourceType:  req.ResourceType,
		SourceSystem:  strings.TrimSpace(req.SourceSystem),
		Justification: strings.TrimSpace(req.Justification),
		RequestedBy:   req.RequestedBy,
		ProposedBy:    req.ProposedBy,
		Payload:       normalizedPayload,
		Metadata:      cloneMap(req.Metadata),
		Risk:          risk,
		Evidence:      evidence,
		Approval:      approval,
		ExpiresAt:     now.Add(24 * time.Hour),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return action, nil
}

func normalizePayload(actionType actiondomain.ActionType, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, newHTTPError(400, "VALIDATION", "payload required")
	}

	switch actionType {
	case actiondomain.ActionTypeWithdrawal:
		var payload actiondomain.WithdrawalPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, newHTTPError(400, "VALIDATION", "payload must be valid json object")
		}
		if strings.TrimSpace(payload.Asset) == "" || strings.TrimSpace(payload.Amount) == "" || strings.TrimSpace(payload.Network) == "" || strings.TrimSpace(payload.DestinationAddress) == "" {
			return nil, newHTTPError(400, "VALIDATION", "withdrawal payload requires asset, amount, network and destination_address")
		}
		return json.Marshal(payload)
	case actiondomain.ActionTypeTreasuryTransfer:
		var payload actiondomain.TreasuryTransferPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, newHTTPError(400, "VALIDATION", "payload must be valid json object")
		}
		if strings.TrimSpace(payload.Asset) == "" || strings.TrimSpace(payload.Amount) == "" || strings.TrimSpace(payload.FromAccount) == "" || strings.TrimSpace(payload.ToAccount) == "" {
			return nil, newHTTPError(400, "VALIDATION", "treasury_transfer payload requires asset, amount, from_account and to_account")
		}
		return json.Marshal(payload)
	case actiondomain.ActionTypeHotToColdMove:
		var payload actiondomain.HotToColdMovePayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, newHTTPError(400, "VALIDATION", "payload must be valid json object")
		}
		if strings.TrimSpace(payload.Asset) == "" || strings.TrimSpace(payload.Amount) == "" || strings.TrimSpace(payload.Network) == "" || strings.TrimSpace(payload.FromWallet) == "" || strings.TrimSpace(payload.ToWallet) == "" {
			return nil, newHTTPError(400, "VALIDATION", "hot_to_cold_move payload requires asset, amount, network, from_wallet and to_wallet")
		}
		return json.Marshal(payload)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", actionType)
	}
}

func riskFor(actionType actiondomain.ActionType, resource actiondomain.ProtectedResource) actiondomain.RiskAssessment {
	resourceFactor := actiondomain.RiskFactor{
		Code:    "resource_criticality",
		Summary: "resource criticality is " + strings.ToLower(strings.TrimSpace(resource.Criticality)),
		Weight:  criticalityWeight(resource.Criticality),
	}
	switch actionType {
	case actiondomain.ActionTypeWithdrawal:
		return actiondomain.RiskAssessment{
			Level:   actiondomain.RiskLevelHigh,
			Score:   80 + resourceFactor.Weight,
			Summary: "withdrawal requires approval",
			Factors: []actiondomain.RiskFactor{
				{Code: "financial_outflow", Summary: "action moves funds out of the protected resource", Weight: 50},
				{Code: "human_approval_required", Summary: "critical action requires manual review", Weight: 30},
				resourceFactor,
			},
		}
	case actiondomain.ActionTypeTreasuryTransfer:
		return actiondomain.RiskAssessment{
			Level:   actiondomain.RiskLevelHigh,
			Score:   76 + resourceFactor.Weight,
			Summary: "treasury transfer requires approval",
			Factors: []actiondomain.RiskFactor{
				{Code: "treasury_move", Summary: "action changes treasury balances", Weight: 46},
				{Code: "human_approval_required", Summary: "critical action requires manual review", Weight: 30},
				resourceFactor,
			},
		}
	default:
		return actiondomain.RiskAssessment{
			Level:   actiondomain.RiskLevelHigh,
			Score:   72 + resourceFactor.Weight,
			Summary: "hot to cold wallet move requires approval",
			Factors: []actiondomain.RiskFactor{
				{Code: "wallet_relocation", Summary: "action relocates funds between custody tiers", Weight: 42},
				{Code: "human_approval_required", Summary: "critical action requires manual review", Weight: 30},
				resourceFactor,
			},
		}
	}
}

func buildEvidence(now time.Time, payload json.RawMessage, resource actiondomain.ProtectedResource) []actiondomain.EvidenceRecord {
	return []actiondomain.EvidenceRecord{
		{
			ID:        uuid.New(),
			Kind:      "payload_validation",
			Status:    actiondomain.EvidenceStatusPassed,
			Summary:   "payload matches the expected action schema",
			Details:   map[string]any{"validated": true, "payload_bytes": len(payload)},
			CreatedAt: now,
		},
		{
			ID:        uuid.New(),
			Kind:      "resource_resolution",
			Status:    actiondomain.EvidenceStatusPassed,
			Summary:   "resource resolved before decision",
			Details:   map[string]any{"resource_id": resource.ID, "resource_type": string(resource.Type), "environment": resource.Environment, "chain": resource.Chain},
			CreatedAt: now,
		},
	}
}

type evaluatedDecision struct {
	effect          actiondomain.Decision
	requireApproval bool
	approvalTTL     time.Duration
	summary         string
	policyID        string
}

func evaluatePolicyDecision(req CreateRequest, resource actiondomain.ProtectedResource, policies []ActionPolicy, evaluator actionPolicyEvaluator) evaluatedDecision {
	defaultDecision := evaluatedDecision{
		effect:          actiondomain.DecisionRequireApproval,
		requireApproval: true,
		approvalTTL:     4 * time.Hour,
		summary:         "no matching policy found; manual approval required by default",
	}
	if evaluator == nil {
		return defaultDecision
	}

	actionAttrs := map[string]any{
		"action_type":   string(req.ActionType),
		"resource_id":   strings.TrimSpace(req.ResourceID),
		"resource_type": string(req.ResourceType),
		"source_system": strings.TrimSpace(req.SourceSystem),
		"justification": strings.TrimSpace(req.Justification),
		"requested_by": map[string]any{
			"type": string(req.RequestedBy.Type),
			"id":   req.RequestedBy.ID,
		},
		"proposed_by": map[string]any{
			"type": string(req.ProposedBy.Type),
			"id":   req.ProposedBy.ID,
		},
		"metadata": cloneMap(req.Metadata),
	}
	resourceAttrs := map[string]any{
		"id":           resource.ID,
		"type":         string(resource.Type),
		"name":         resource.Name,
		"environment":  resource.Environment,
		"chain":        resource.Chain,
		"criticality":  resource.Criticality,
		"labels":       cloneStringMap(resource.Labels),
	}

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		matched, err := evaluator.Matches(policy.Expression, actionAttrs, resourceAttrs)
		if err != nil || !matched {
			continue
		}
		if policy.Effect == "deny" {
			return evaluatedDecision{
				effect:      actiondomain.DecisionDeny,
				approvalTTL: 0,
				summary:     firstNonEmpty(policy.Reason, "matching deny policy blocked the action"),
				policyID:    policy.ID,
			}
		}
		if policy.RequireApproval {
			return evaluatedDecision{
				effect:          actiondomain.DecisionRequireApproval,
				requireApproval: true,
				approvalTTL:     approvalTTL(policy.ApprovalTTLSeconds),
				summary:         firstNonEmpty(policy.Reason, "matching allow policy requires approval"),
				policyID:        policy.ID,
			}
		}
		return evaluatedDecision{
			effect:      actiondomain.DecisionAllow,
			approvalTTL: 0,
			summary:     firstNonEmpty(policy.Reason, "matching allow policy approved the action"),
			policyID:    policy.ID,
		}
	}

	return defaultDecision
}

func approvalTTL(seconds int) time.Duration {
	if seconds <= 0 {
		return 4 * time.Hour
	}
	return time.Duration(seconds) * time.Second
}

func criticalityWeight(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return 15
	case "high":
		return 10
	case "medium":
		return 5
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
