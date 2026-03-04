package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	policydomain "nexus-core/internal/policy/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
	"nexus/pkg/validations/jsonschema"
)

// simulateState agrupa estado para el pipeline Simulate.
type simulateState struct {
	requestID  string
	input      map[string]any
	contextMap map[string]any
	explain    map[string]any
	start      time.Time
}

func (u *Usecases) initSimulateState(req gwdomain.RunRequest) simulateState {
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.NewString()
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	contextMap := req.Context
	if contextMap == nil {
		contextMap = map[string]any{}
	}
	return simulateState{
		requestID:  requestID,
		input:      input,
		contextMap: contextMap,
		explain:    map[string]any{"mode": "simulate"},
		start:      time.Now(),
	}
}

// simulateDeny construye SimulateResponse para decisión deny.
func (u *Usecases) simulateDeny(st simulateState, tool tooldomain.Tool, reason string, code string, httpStatus int, stage string) gwdomain.SimulateResponse {
	if stage != "" {
		st.explain["stage"] = stage
	}
	return gwdomain.SimulateResponse{
		RequestID:  st.requestID,
		Decision:   gwdomain.DecisionDeny,
		ToolName:   tool.Name,
		Status:     gwdomain.RunStatusBlocked,
		Reason:     &reason,
		ErrorCode:  &code,
		ErrorMsg:   &reason,
		LatencyMS:  time.Since(st.start).Milliseconds(),
		HTTPStatus: httpStatus,
		Explain:    st.explain,
	}
}

// simulateAllow construye SimulateResponse para decisión allow.
func (u *Usecases) simulateAllow(st simulateState, tool tooldomain.Tool, matchReason string) gwdomain.SimulateResponse {
	return gwdomain.SimulateResponse{
		RequestID:  st.requestID,
		Decision:   gwdomain.DecisionAllow,
		ToolName:   tool.Name,
		Status:     gwdomain.RunStatusSuccess,
		Reason:     strPtr(matchReason),
		LatencyMS:  time.Since(st.start).Milliseconds(),
		HTTPStatus: http.StatusOK,
		Explain:    st.explain,
	}
}

// simulateEnrichContext añade actor, role, scopes al context (compartido con Run).
func (u *Usecases) simulateEnrichContext(req gwdomain.RunRequest, st *simulateState) {
	if req.Actor != nil && *req.Actor != "" {
		st.contextMap["actor"] = *req.Actor
	}
	if req.Role != nil && *req.Role != "" {
		st.contextMap["role"] = *req.Role
	}
	if len(req.Scopes) > 0 {
		arr := make([]any, 0, len(req.Scopes))
		for _, scope := range req.Scopes {
			arr = append(arr, scope)
		}
		st.contextMap["scopes"] = arr
	}
}

// simulateSchemaCheck valida schema de entrada; devuelve deny response si falla.
func (u *Usecases) simulateSchemaCheck(ctx context.Context, st *simulateState, tool tooldomain.Tool) *gwdomain.SimulateResponse {
	inSchema, err := u.cache.Compile(ctx, tool.ID.String()+":in", tool.InputSchemaJSON)
	if err != nil {
		resp := u.simulateDeny(*st, tool, "tool input schema invalid", types.ErrCodeSchemaInvalid, http.StatusForbidden, "schema_compile")
		return &resp
	}
	if err := jsonschema.Validate(inSchema, st.input); err != nil {
		resp := u.simulateDeny(*st, tool, "input does not match schema", types.ErrCodeValidation, http.StatusBadRequest, "schema_validate")
		return &resp
	}
	return nil
}

// simulatePoliciesAndLimits evalúa políticas y límites de tamaño.
func (u *Usecases) simulatePoliciesAndLimits(ctx context.Context, orgID uuid.UUID, st *simulateState, tool tooldomain.Tool) (decision gwdomain.Decision, matchReason string, policyID *uuid.UUID) {
	policies, err := u.policyRepo.ListByToolID(ctx, orgID, tool.ID)
	if err != nil {
		return gwdomain.DecisionDeny, "policy lookup failed", nil
	}
	match, matchReason, limits, err := u.firstMatch(policies, st.input, st.contextMap, tool)
	if err != nil {
		return gwdomain.DecisionDeny, "policy evaluation failed", nil
	}
	decision = gwdomain.DecisionAllow
	if match != nil {
		id := match.ID
		policyID = &id
		st.explain["matched_policy_id"] = id.String()
		st.explain["matched_policy_effect"] = string(match.Effect)
		st.explain["matched_policy_priority"] = match.Priority
		if match.Effect == policydomain.EffectDeny {
			decision = gwdomain.DecisionDeny
		}
	} else if tool.ActionType == tooldomain.ActionWrite {
		decision = gwdomain.DecisionDeny
		matchReason = "default deny for write tool"
		st.explain["default_decision"] = "deny"
	} else {
		matchReason = "default allow for read tool"
		st.explain["default_decision"] = "allow"
	}
	if decision == gwdomain.DecisionAllow {
		if maxIn := limits.maxBytesInput(u.cfg.MaxBytesInputDefault); maxIn > 0 {
			n, _ := utils.JSONSize(st.input)
			st.explain["input_bytes"] = n
			st.explain["max_bytes_input"] = maxIn
			if n > maxIn {
				decision = gwdomain.DecisionDeny
				matchReason = "input too large"
			}
		}
		if maxCtx := limits.maxBytesContext(u.cfg.MaxBytesContextDefault); maxCtx > 0 {
			n, _ := utils.JSONSize(st.contextMap)
			st.explain["context_bytes"] = n
			st.explain["max_bytes_context"] = maxCtx
			if n > maxCtx {
				decision = gwdomain.DecisionDeny
				matchReason = "context too large"
			}
		}
	}
	return decision, matchReason, policyID
}

// simulateEgressCheck verifica egress allowlist.
func (u *Usecases) simulateEgressCheck(ctx context.Context, orgID uuid.UUID, st *simulateState, tool tooldomain.Tool) (allowed bool, err error) {
	parsed, parseErr := url.Parse(tool.URL)
	if parseErr != nil || parsed.Hostname() == "" {
		return false, nil
	}
	host := strings.ToLower(parsed.Hostname())
	st.explain["egress_host"] = host
	allowed, err = u.egress.IsHostAllowed(ctx, orgID, tool.ID, host)
	if err != nil {
		return false, err
	}
	st.explain["egress_allowed"] = allowed
	return allowed, nil
}
