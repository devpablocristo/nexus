package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	gwdomain "data-plane/internal/gateway/usecases/domain"
	tooldomain "data-plane/internal/tool/usecases/domain"
	"nexus/pkg/types"
)

func classifyRiskClass(tool tooldomain.Tool, input, contextMap map[string]any, approvalRequired bool) gwdomain.RiskClass {
	method := strings.ToUpper(strings.TrimSpace(tool.Method))
	if explicitBreakGlassIntent(input, contextMap) {
		return gwdomain.RiskClassBreakGlass
	}
	if explicitPlanIntent(input, contextMap, tool.Name) {
		return gwdomain.RiskClassPlan
	}
	if method == http.MethodDelete {
		if approvalRequired || isProductionTarget(input, contextMap) || tool.RiskLevel >= 4 || strings.EqualFold(tool.Sensitivity, "high") {
			return gwdomain.RiskClassDestructiveProd
		}
		return gwdomain.RiskClassMutateNonProd
	}
	if tool.ActionType == tooldomain.ActionRead {
		return gwdomain.RiskClassRead
	}
	if approvalRequired || isProductionTarget(input, contextMap) || tool.RiskLevel >= 4 || strings.EqualFold(tool.Sensitivity, "high") || strings.EqualFold(tool.Classification, "external") {
		return gwdomain.RiskClassMutateProd
	}
	return gwdomain.RiskClassMutateNonProd
}

func explicitPlanIntent(input, contextMap map[string]any, toolName string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(toolName)), "plan") {
		return true
	}
	for _, m := range []map[string]any{input, contextMap} {
		if m == nil {
			continue
		}
		if boolLike(m["dry_run"]) || boolLike(m["plan_only"]) {
			return true
		}
		for _, key := range []string{"mode", "intent", "operation"} {
			if looksLikePlan(asStringLower(m[key])) {
				return true
			}
		}
	}
	return false
}

func explicitBreakGlassIntent(input, contextMap map[string]any) bool {
	for _, m := range []map[string]any{input, contextMap} {
		if m == nil {
			continue
		}
		if boolLike(m["break_glass"]) || boolLike(m["emergency_access"]) {
			return true
		}
		for _, key := range []string{"mode", "intent", "operation", "approval_mode"} {
			if looksLikeBreakGlass(asStringLower(m[key])) {
				return true
			}
		}
	}
	return false
}

func isProductionTarget(input, contextMap map[string]any) bool {
	for _, m := range []map[string]any{input, contextMap} {
		if m == nil {
			continue
		}
		for _, key := range []string{"env", "environment", "target_env", "target_environment", "workspace"} {
			if looksLikeProd(asStringLower(m[key])) {
				return true
			}
		}
	}
	return false
}

func boolLike(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func asStringLower(v any) string {
	switch t := v.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return ""
	}
}

func looksLikeProd(s string) bool {
	return s == "prod" || s == "production" || strings.HasPrefix(s, "prod-")
}

func looksLikePlan(s string) bool {
	return s == "plan" || s == "preview" || s == "dry-run" || s == "dry_run"
}

func looksLikeBreakGlass(s string) bool {
	return s == "break_glass" || s == "break-glass" || s == "emergency" || s == "emergency_override"
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		out := make(map[string]any, len(in))
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		out = make(map[string]any, len(in))
		for k, v := range in {
			out[k] = v
		}
	}
	return out
}

func (u *Usecases) ExecuteIntent(ctx context.Context, orgID, intentID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error) {
	return u.ExecuteIntentWithLease(ctx, orgID, intentID, uuid.Nil, timeoutMS)
}

func (u *Usecases) ExecuteIntentWithLease(ctx context.Context, orgID, intentID, leaseID uuid.UUID, timeoutMS int) (gwdomain.RunResponse, error) {
	if u.intentRepo == nil {
		return gwdomain.RunResponse{}, types.NewHTTPError(http.StatusNotImplemented, types.ErrCodeValidation, "execution intents are not configured")
	}
	intent, err := u.intentRepo.GetByID(ctx, orgID, intentID)
	if err != nil {
		return gwdomain.RunResponse{}, err
	}
	if intent.Status != gwdomain.IntentStatusApproved {
		reason := "intent is not approved for execution"
		code := types.ErrCodeApprovalRequired
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    uuidStringPtrOrNil(leaseID),
		}, nil
	}
	if !intent.ExpiresAt.IsZero() && time.Now().UTC().After(intent.ExpiresAt) {
		reason := "intent expired before execution"
		code := types.ErrCodeApprovalRequired
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    uuidStringPtrOrNil(leaseID),
		}, nil
	}
	if intent.PreflightStatus == gwdomain.PreflightStatusFailed && intent.RiskClass != gwdomain.RiskClassBreakGlass {
		reason := "intent preflight failed and cannot be executed"
		code := types.ErrCodePreflightFailed
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    uuidStringPtrOrNil(leaseID),
		}, nil
	}
	if u.leaseRepo == nil || leaseID == uuid.Nil {
		reason := "execution lease required before executing intent"
		code := types.ErrCodeLeaseRequired
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
		}, nil
	}
	lease, err := u.leaseRepo.GetByID(ctx, orgID, leaseID)
	if err != nil {
		reason := "execution lease not found"
		code := types.ErrCodeLeaseInvalid
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    uuidStringPtrOrNil(leaseID),
		}, nil
	}
	if lease.IntentID != intent.ID || lease.Status != gwdomain.ExecutionLeaseStatusActive {
		reason := "execution lease is not active for this intent"
		code := types.ErrCodeLeaseInvalid
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    strPtr(lease.ID.String()),
		}, nil
	}
	if time.Now().UTC().After(lease.ExpiresAt) {
		_ = u.leaseRepo.MarkExpired(ctx, orgID, lease.ID)
		reason := "execution lease expired before execution"
		code := types.ErrCodeLeaseExpired
		return gwdomain.RunResponse{
			RequestID:  uuid.NewString(),
			Decision:   gwdomain.DecisionDeny,
			ToolName:   intent.ToolName,
			Status:     gwdomain.RunStatusBlocked,
			Reason:     &reason,
			ErrorCode:  &code,
			ErrorMsg:   &reason,
			HTTPStatus: http.StatusForbidden,
			IntentID:   strPtr(intent.ID.String()),
			RiskClass:  strPtr(string(intent.RiskClass)),
			ApprovalID: uuidStringPtr(intent.ApprovalID),
			LeaseID:    strPtr(lease.ID.String()),
		}, nil
	}
	_ = u.leaseRepo.MarkUsed(ctx, orgID, lease.ID)

	contextMap := cloneMap(intent.Context)
	contextMap["intent_id"] = intent.ID.String()
	contextMap["execution_lease"] = map[string]any{
		"lease_id":         lease.ID.String(),
		"credential_mode":  lease.CredentialMode,
		"credential_hints": cloneMap(lease.CredentialHints),
		"expires_at":       lease.ExpiresAt.UTC().Format(time.RFC3339),
	}
	req := gwdomain.RunRequest{
		RequestID:      uuid.NewString(),
		ToolName:       intent.ToolName,
		ToolID:         intent.ToolID.String(),
		IntentID:       intent.ID.String(),
		ExecutionLease: &lease,
		Input:          cloneMap(intent.Input),
		Context:        contextMap,
		Actor:          intent.Actor,
		Role:           intent.Role,
		Scopes:         append([]string{}, intent.Scopes...),
		TimeoutMS:      timeoutMS,
		RequestSource:  "intent_execute",
	}
	resp, err := u.Run(ctx, orgID, req)
	if err != nil {
		return resp, err
	}
	resp.IntentID = strPtr(intent.ID.String())
	resp.RiskClass = strPtr(string(intent.RiskClass))
	resp.ApprovalID = uuidStringPtr(intent.ApprovalID)
	resp.LeaseID = strPtr(lease.ID.String())
	if resp.Status == gwdomain.RunStatusSuccess || resp.Status == gwdomain.RunStatusError {
		_ = u.intentRepo.MarkExecuted(ctx, orgID, intent.ID)
	}
	return resp, nil
}

func (u *Usecases) IssueExecutionLease(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.ExecutionLease, error) {
	if u.intentRepo == nil || u.leaseRepo == nil {
		return gwdomain.ExecutionLease{}, types.NewHTTPError(http.StatusNotImplemented, types.ErrCodeValidation, "execution leases are not configured")
	}
	intent, err := u.intentRepo.GetByID(ctx, orgID, intentID)
	if err != nil {
		return gwdomain.ExecutionLease{}, err
	}
	if intent.Status != gwdomain.IntentStatusApproved {
		return gwdomain.ExecutionLease{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeApprovalRequired, "intent must be approved before issuing a lease")
	}
	if !intent.ExpiresAt.IsZero() && time.Now().UTC().After(intent.ExpiresAt) {
		return gwdomain.ExecutionLease{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeLeaseExpired, "intent expired before lease issuance")
	}
	if intent.PreflightStatus == gwdomain.PreflightStatusFailed && intent.RiskClass != gwdomain.RiskClassBreakGlass {
		return gwdomain.ExecutionLease{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodePreflightFailed, "intent preflight failed and cannot receive a lease")
	}
	ttlSeconds := executionLeaseTTLSeconds(intent.RiskClass)
	credentialMode, credentialHints := executionLeaseCredentialSpec(intent)
	return u.leaseRepo.Create(ctx, gwdomain.ExecutionLease{
		OrgID:           orgID,
		IntentID:        intent.ID,
		ToolName:        intent.ToolName,
		RiskClass:       intent.RiskClass,
		Status:          gwdomain.ExecutionLeaseStatusActive,
		CredentialMode:  credentialMode,
		CredentialHints: credentialHints,
		ExpiresAt:       time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second),
	})
}

func (u *Usecases) GetIntent(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.ExecutionIntent, error) {
	if u.intentRepo == nil {
		return gwdomain.ExecutionIntent{}, types.NewHTTPError(http.StatusNotImplemented, types.ErrCodeValidation, "execution intents are not configured")
	}
	return u.intentRepo.GetByID(ctx, orgID, intentID)
}

func (u *Usecases) ListIntents(ctx context.Context, orgID uuid.UUID, limit int) ([]gwdomain.ExecutionIntent, error) {
	if u.intentRepo == nil {
		return nil, types.NewHTTPError(http.StatusNotImplemented, types.ErrCodeValidation, "execution intents are not configured")
	}
	return u.intentRepo.ListRecent(ctx, orgID, limit)
}

func (u *Usecases) GetIntentPreflight(ctx context.Context, orgID, intentID uuid.UUID) (gwdomain.PreflightReview, error) {
	intent, err := u.GetIntent(ctx, orgID, intentID)
	if err != nil {
		return gwdomain.PreflightReview{}, err
	}
	return gwdomain.PreflightReview{
		IntentID:       intent.ID,
		ToolName:       intent.ToolName,
		RiskClass:      intent.RiskClass,
		Reason:         intent.Reason,
		Status:         intent.PreflightStatus,
		Summary:        cloneMap(intent.PreflightSummary),
		ArtifactSHA256: intent.PreflightArtifactSHA,
		CompletedAt:    intent.PreflightCompletedAt,
		ApprovalID:     intent.ApprovalID,
		IntentStatus:   intent.Status,
	}, nil
}

func uuidStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func uuidStringPtrOrNil(id uuid.UUID) *string {
	if id == uuid.Nil {
		return nil
	}
	s := id.String()
	return &s
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

func executionLeaseCredentialSpec(intent gwdomain.ExecutionIntent) (string, map[string]any) {
	targetEnv := firstString(intent.Input, intent.Context, "target_env", "target_environment", "environment", "env")
	toolName := strings.ToLower(intent.ToolName)
	hints := map[string]any{
		"intent_id":    intent.ID.String(),
		"tool_name":    intent.ToolName,
		"risk_class":   string(intent.RiskClass),
		"target_env":   targetEnv,
		"session_tags": map[string]any{"intent_id": intent.ID.String(), "risk_class": string(intent.RiskClass)},
	}
	switch {
	case strings.Contains(toolName, "terraform") || strings.Contains(toolName, "aws"):
		hints["provider"] = "aws"
		hints["scope"] = "sts_assume_role"
		return "aws_sts", hints
	case strings.Contains(toolName, "kubectl"):
		hints["provider"] = "kubernetes"
		hints["scope"] = "ephemeral_kubeconfig"
		return "kubeconfig_ephemeral", hints
	case strings.Contains(toolName, "bash") || strings.Contains(toolName, "shell"):
		hints["provider"] = "shell"
		hints["scope"] = "lease_bound_session"
		return "session_bound", hints
	default:
		hints["provider"] = "generic"
		hints["scope"] = "lease_only"
		return "lease_only", hints
	}
}
