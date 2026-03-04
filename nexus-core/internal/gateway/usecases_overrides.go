package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	gwdomain "nexus-core/internal/gateway/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
)

// checkActionOverrides aplica overrides de runtime; devuelve respuesta si hay deny.
func (u *Usecases) checkActionOverrides(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	st.runtimeOverrides = RuntimeActionOverrides{}
	if u.actionOverrides == nil {
		return nil, nil
	}
	var err error
	st.runtimeOverrides, err = u.actionOverrides.ResolveRuntimeOverrides(ctx, orgID, st.tool.Name)
	if err != nil {
		return nil, err
	}
	if !st.runtimeOverrides.Deny {
		return nil, nil
	}
	reason := st.runtimeOverrides.DenyReason
	if strings.TrimSpace(reason) == "" {
		reason = "blocked by active action override"
	}
	resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, types.ErrCodePolicyDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
	u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
	return &resp, nil
}

// checkTenantRateLimit verifica rate limit por tenant.
func (u *Usecases) checkTenantRateLimit(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	tenantRunRPM := 0
	if u.tenantCaps != nil {
		var err error
		tenantRunRPM, err = u.tenantCaps.GetRunRPM(ctx, orgID)
		if err != nil {
			return nil, err
		}
	}
	if st.runtimeOverrides.TenantRPMOverride != nil && *st.runtimeOverrides.TenantRPMOverride > 0 {
		if tenantRunRPM <= 0 || *st.runtimeOverrides.TenantRPMOverride < tenantRunRPM {
			tenantRunRPM = *st.runtimeOverrides.TenantRPMOverride
		}
	}
	if tenantRunRPM <= 0 {
		return nil, nil
	}
	tenantKey := orgID.String() + ":tenant"
	if u.limiter.Allow(tenantKey, tenantRunRPM) {
		return nil, nil
	}
	reason := "tenant run rate limit exceeded (bucket=org:tenant limit_per_minute=" + strconv.Itoa(tenantRunRPM) + ")"
	resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, types.ErrCodeRateLimited, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
	u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
	return &resp, nil
}

// checkToolRateLimit verifica rate limit por tool.
func (u *Usecases) checkToolRateLimit(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState) (*gwdomain.RunResponse, error) {
	perMin := st.limits.rateLimitPerMinute(u.cfg.DefaultRateLimitPerMinute)
	if st.runtimeOverrides.ToolRPMOverride != nil && *st.runtimeOverrides.ToolRPMOverride > 0 {
		if perMin <= 0 || *st.runtimeOverrides.ToolRPMOverride < perMin {
			perMin = *st.runtimeOverrides.ToolRPMOverride
		}
	}
	if perMin <= 0 {
		return nil, nil
	}
	key := orgID.String() + ":" + st.tool.Name
	if u.limiter.Allow(key, perMin) {
		return nil, nil
	}
	reason := "rate limit exceeded (bucket=org+tool limit_per_minute=" + strconv.Itoa(perMin) + ")"
	resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, st.tool.Name, st.tool.ID, st.policyID, reason, types.ErrCodeRateLimited, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
	u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, st.tool.Name, st.idempotencyKey, &resp)
	return &resp, nil
}

// validateURLAndEgress valida URL de la tool y egress allowlist.
func (u *Usecases) validateURLAndEgress(ctx context.Context, orgID uuid.UUID, req gwdomain.RunRequest, st *runState, tool tooldomain.Tool) (*gwdomain.RunResponse, error) {
	parsed, parseErr := url.Parse(tool.URL)
	if parseErr != nil || parsed.Hostname() == "" {
		resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, tool.Name, tool.ID, st.policyID, "invalid tool url", types.ErrCodeValidation, http.StatusBadRequest, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
		u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, tool.Name, st.idempotencyKey, &resp)
		return &resp, nil
	}
	if !u.cfg.DisableSSRFProtection {
		if err := utils.ValidateEgressURLWithAllowlist(tool.URL, u.cfg.EgressAllowlist); err != nil {
			resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, tool.Name, tool.ID, st.policyID, "ssrf blocked: "+err.Error(), types.ErrCodeEgressDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
			u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, tool.Name, st.idempotencyKey, &resp)
			return &resp, nil
		}
	}
	allowed, err := u.egress.IsHostAllowed(ctx, orgID, tool.ID, strings.ToLower(parsed.Hostname()))
	if err != nil {
		return nil, err
	}
	if allowed {
		return nil, nil
	}
	resp := u.blocked(ctx, orgID, req.Actor, req.Role, req.Scopes, st.requestID, tool.Name, tool.ID, st.policyID, "egress host denied", types.ErrCodeEgressDenied, http.StatusForbidden, st.start, st.input, st.contextMap, st.idemMeta, req.TimeoutMS, intPtr(st.budget.RemainingMS()), st.budget.StageDurationsMS())
	u.failIdempotencyIfNeeded(ctx, st.createdIdempotencyRow, orgID, tool.Name, st.idempotencyKey, &resp)
	return &resp, nil
}

// resolveSecrets carga secretos de la tool y popula st.headers.
func (u *Usecases) resolveSecrets(ctx context.Context, orgID uuid.UUID, st *runState) error {
	st.headers = map[string]string{}
	secrets, err := u.secretRepo.ListForTool(ctx, orgID, st.tool.ID)
	if err != nil {
		return err
	}
	for _, secret := range secrets {
		if !secret.Enabled {
			continue
		}
		if strings.EqualFold(secret.SecretType, "header") && secret.KeyName != "" {
			st.headers[secret.KeyName] = secret.PlaintextValue
		}
		if strings.EqualFold(secret.SecretType, "bearer") {
			st.headers["Authorization"] = "Bearer " + secret.PlaintextValue
		}
	}
	st.headers["X-Nexus-Request-Id"] = st.requestID
	return nil
}
