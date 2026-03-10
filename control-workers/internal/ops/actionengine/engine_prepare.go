package actionengine

import (
	"context"
	"strings"

	"github.com/google/uuid"
	actiondomain "control-workers/internal/ops/actionengine/usecases/domain"
	"nexus/pkg/types"
)

// validatePrepareInput valida action_type, scope, params y ttl_seconds.
func (e *engine) validatePrepareInput(req EngineRequest) (preparedInput, error) {
	actionType := strings.TrimSpace(req.ActionType)
	if actionType == "" {
		return preparedInput{}, types.NewHTTPError(400, types.ErrCodeValidation, "action_type is required")
	}
	if req.Scope == nil {
		return preparedInput{}, types.NewHTTPError(400, types.ErrCodeValidation, "scope is required")
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	if req.TTLSeconds <= 0 {
		return preparedInput{}, types.NewHTTPError(400, types.ErrCodeValidation, "ttl_seconds must be > 0")
	}
	return preparedInput{}, types.NewHTTPError(0, "", "") // sentinel to continue
}

// resolveScopeAndHashes canonicaliza scope/params y calcula hashes.
func (e *engine) resolveScopeAndHashes(req EngineRequest) (scopeCanonical map[string]any, paramsCanonical map[string]any, scopeHash, paramsHash, idempotencyKey string, err error) {
	scopeCanonical, err = canonicalizeScope(req.Scope)
	if err != nil {
		return nil, nil, "", "", "", types.NewHTTPError(400, types.ErrCodeValidation, err.Error())
	}
	paramsCanonical = normalizeJSON(req.Params, true).(map[string]any)
	scopeHash, err = hashCanonical(scopeCanonical)
	if err != nil {
		return nil, nil, "", "", "", err
	}
	paramsHash, err = hashCanonical(paramsCanonical)
	if err != nil {
		return nil, nil, "", "", "", err
	}
	incidentPart := ""
	if req.IncidentID != nil {
		incidentPart = req.IncidentID.String()
	}
	idempotencyKey = hashString(incidentPart + "|" + strings.TrimSpace(req.ActionType) + "|" + scopeHash + "|" + paramsHash)
	return scopeCanonical, paramsCanonical, scopeHash, paramsHash, idempotencyKey, nil
}

// resolveMaxTTL obtiene el TTL máximo permitido (catalog, tenant).
func (e *engine) resolveMaxTTL(ctx context.Context, orgID uuid.UUID, catalog actiondomain.CatalogItem, req EngineRequest) int {
	maxTTL := e.cfg.DefaultMaxTTL
	if catalog.MaxTTLSeconds > 0 {
		maxTTL = catalog.MaxTTLSeconds
	}
	if e.tenant != nil {
		if profile, err := e.tenant.GetProfile(ctx, orgID); err == nil && profile.MaxTTLSeconds > 0 && profile.MaxTTLSeconds < maxTTL {
			maxTTL = profile.MaxTTLSeconds
		}
	}
	return maxTTL
}
