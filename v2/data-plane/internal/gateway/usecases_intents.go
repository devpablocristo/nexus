package gateway

import (
	"net/http"
	"strings"
	"time"

	gwdomain "nexus/v2/data-plane/internal/gateway/usecases/domain"
	"nexus/v2/data-plane/internal/tool"
)

type preflightEvaluation struct {
	required      bool
	status        gwdomain.PreflightStatus
	summary       map[string]any
	failureReason string
	failureHTTP   int
}

func classifyRiskClass(toolDef tool.Definition, input, contextMap map[string]any) gwdomain.RiskClass {
	method := strings.ToUpper(strings.TrimSpace(toolDef.Method))

	switch {
	case explicitBreakGlassIntent(input, contextMap):
		return gwdomain.RiskClassBreakGlass
	case explicitPlanIntent(input, contextMap, toolDef.Name):
		return gwdomain.RiskClassPlan
	case method == http.MethodGet:
		return gwdomain.RiskClassRead
	case method == http.MethodDelete && isProductionTarget(input, contextMap):
		return gwdomain.RiskClassDestructiveProd
	case isProductionTarget(input, contextMap):
		return gwdomain.RiskClassMutateProd
	default:
		return gwdomain.RiskClassMutateNonProd
	}
}

func evaluateDeterministicPreflight(riskClass gwdomain.RiskClass, input, contextMap map[string]any) preflightEvaluation {
	switch riskClass {
	case gwdomain.RiskClassRead, gwdomain.RiskClassPlan, gwdomain.RiskClassMutateNonProd:
		return preflightEvaluation{
			status:  gwdomain.PreflightStatusNotRequired,
			summary: map[string]any{"required": false},
		}
	case gwdomain.RiskClassMutateProd:
		hasTicket := hasAnyNonEmptyString(input, contextMap, "change_ticket", "ticket", "ticket_id")
		summary := map[string]any{
			"required": true,
			"checks": map[string]any{
				"change_ticket": hasTicket,
			},
		}
		if hasTicket {
			return preflightEvaluation{
				required: true,
				status:   gwdomain.PreflightStatusPassed,
				summary:  summary,
			}
		}
		return preflightEvaluation{
			required:      true,
			status:        gwdomain.PreflightStatusFailed,
			summary:       summary,
			failureReason: "preflight requires change_ticket for production execution",
			failureHTTP:   http.StatusForbidden,
		}
	case gwdomain.RiskClassDestructiveProd:
		hasTicket := hasAnyNonEmptyString(input, contextMap, "change_ticket", "ticket", "ticket_id")
		hasRestoreEvidence := hasAnyNonEmptyString(input, contextMap, "restore_evidence", "rollback_plan", "restore_plan") ||
			hasAnyTrueBool(input, contextMap, "restore_ready", "rollback_ready")
		summary := map[string]any{
			"required": true,
			"checks": map[string]any{
				"change_ticket":    hasTicket,
				"restore_evidence": hasRestoreEvidence,
			},
		}
		if hasTicket && hasRestoreEvidence {
			return preflightEvaluation{
				required: true,
				status:   gwdomain.PreflightStatusPassed,
				summary:  summary,
			}
		}
		return preflightEvaluation{
			required:      true,
			status:        gwdomain.PreflightStatusFailed,
			summary:       summary,
			failureReason: "preflight requires change_ticket and restore evidence for destructive production execution",
			failureHTTP:   http.StatusForbidden,
		}
	case gwdomain.RiskClassBreakGlass:
		hasIncident := hasAnyNonEmptyString(input, contextMap, "incident_id", "incident", "incident_ref")
		hasJustification := hasAnyNonEmptyString(input, contextMap, "justification", "reason")
		summary := map[string]any{
			"required": true,
			"checks": map[string]any{
				"incident_id":   hasIncident,
				"justification": hasJustification,
			},
		}
		if hasIncident && hasJustification {
			return preflightEvaluation{
				required: true,
				status:   gwdomain.PreflightStatusPassed,
				summary:  summary,
			}
		}
		return preflightEvaluation{
			required:      true,
			status:        gwdomain.PreflightStatusFailed,
			summary:       summary,
			failureReason: "preflight requires incident_id and justification for break-glass execution",
			failureHTTP:   http.StatusForbidden,
		}
	default:
		return preflightEvaluation{
			status:  gwdomain.PreflightStatusNotRequired,
			summary: map[string]any{"required": false},
		}
	}
}

func explicitPlanIntent(input, contextMap map[string]any, toolName string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(toolName)), "plan") {
		return true
	}
	for _, item := range []map[string]any{input, contextMap} {
		if item == nil {
			continue
		}
		if boolLike(item["dry_run"]) || boolLike(item["plan_only"]) {
			return true
		}
		for _, key := range []string{"mode", "intent", "operation"} {
			if looksLikePlan(asStringLower(item[key])) {
				return true
			}
		}
	}
	return false
}

func explicitBreakGlassIntent(input, contextMap map[string]any) bool {
	for _, item := range []map[string]any{input, contextMap} {
		if item == nil {
			continue
		}
		if boolLike(item["break_glass"]) || boolLike(item["emergency_access"]) {
			return true
		}
		for _, key := range []string{"mode", "intent", "operation", "approval_mode"} {
			if looksLikeBreakGlass(asStringLower(item[key])) {
				return true
			}
		}
	}
	return false
}

func isProductionTarget(input, contextMap map[string]any) bool {
	for _, item := range []map[string]any{input, contextMap} {
		if item == nil {
			continue
		}
		for _, key := range []string{"env", "environment", "target_env", "target_environment", "workspace"} {
			if looksLikeProd(asStringLower(item[key])) {
				return true
			}
		}
	}
	return false
}

func boolLike(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func asStringLower(v any) string {
	if typed, ok := v.(string); ok {
		return strings.ToLower(strings.TrimSpace(typed))
	}
	return ""
}

func looksLikeProd(value string) bool {
	return value == "prod" || value == "production" || strings.HasPrefix(value, "prod-")
}

func looksLikePlan(value string) bool {
	return value == "plan" || value == "preview" || value == "dry-run" || value == "dry_run"
}

func looksLikeBreakGlass(value string) bool {
	return value == "break_glass" || value == "break-glass" || value == "emergency" || value == "emergency_override"
}

func hasAnyNonEmptyString(input, contextMap map[string]any, keys ...string) bool {
	return firstNonEmptyString(input, contextMap, keys...) != ""
}

func hasAnyTrueBool(input, contextMap map[string]any, keys ...string) bool {
	for _, item := range []map[string]any{input, contextMap} {
		if item == nil {
			continue
		}
		for _, key := range keys {
			if boolLike(item[key]) {
				return true
			}
		}
	}
	return false
}

func firstNonEmptyString(input, contextMap map[string]any, keys ...string) string {
	for _, item := range []map[string]any{input, contextMap} {
		if item == nil {
			continue
		}
		for _, key := range keys {
			if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func nowPtrIfRequired(required bool) *time.Time {
	if !required {
		return nil
	}
	now := time.Now().UTC()
	return &now
}

func executionLeaseTTLSeconds(riskClass gwdomain.RiskClass) int {
	switch riskClass {
	case gwdomain.RiskClassDestructiveProd, gwdomain.RiskClassBreakGlass:
		return 300
	case gwdomain.RiskClassMutateProd:
		return 600
	default:
		return 900
	}
}

func executionLeaseCredentialMode(intent gwdomain.ExecutionIntent) string {
	toolName := strings.ToLower(intent.ToolName)
	switch {
	case strings.Contains(toolName, "terraform"), strings.Contains(toolName, "aws"):
		return "aws_sts"
	case strings.Contains(toolName, "kubectl"):
		return "kube_token"
	default:
		return "none"
	}
}

func executionLeaseCredentialHints(intent gwdomain.ExecutionIntent) map[string]any {
	targetEnv := firstNonEmptyString(intent.Input, intent.Context, "target_env", "target_environment", "environment", "env")
	return map[string]any{
		"intent_id":  intent.ID.String(),
		"tool_name":  intent.ToolName,
		"risk_class": string(intent.RiskClass),
		"target_env": targetEnv,
	}
}
