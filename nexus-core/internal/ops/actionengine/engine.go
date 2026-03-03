package actionengine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	actiondomain "nexus-core/internal/ops/actionengine/usecases/domain"
	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
	tenantdomain "nexus-core/internal/ops/tenant/usecases/domain"
	"nexus-core/pkg/types"
	"nexus-core/pkg/validations/jsonschema"
)

type EventEmitterPort interface {
	Emit(ctx context.Context, in opseventstore.EmitInput) (opsdomain.StoredEvent, error)
}

type TenantPort interface {
	GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error)
}

type EngineConfig struct {
	ActionSchemaDir string
	DefaultMaxTTL   int
}

type EngineRequest struct {
	IncidentID      *uuid.UUID      `json:"incident_id,omitempty"`
	ProposalID      *uuid.UUID      `json:"proposal_id,omitempty"`
	ActionType      string          `json:"action_type,omitempty"`
	Scope           map[string]any  `json:"scope,omitempty"`
	TTLSeconds      int             `json:"ttl_seconds,omitempty"`
	Params          map[string]any  `json:"params,omitempty"`
	EvidenceRefs    []string        `json:"evidence_refs,omitempty"`
	ApprovalGranted bool            `json:"approval_granted,omitempty"`
	ApprovalComment *string         `json:"approval_comment,omitempty"`
}

type EngineResult struct {
	Proposal        actiondomain.Proposal  `json:"proposal"`
	Execution       *actiondomain.Execution `json:"execution,omitempty"`
	IdempotencyKey  string                  `json:"idempotency_key"`
	ScopeHash       string                  `json:"scope_hash"`
	ParamsHash      string                  `json:"params_hash"`
	ApprovalRequired bool                   `json:"approval_required"`
	Replay          bool                    `json:"replay"`
}

type Engine interface {
	DryRun(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error)
	Apply(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error)
	Rollback(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error)
}

type engineRepoPort interface {
	UpsertCatalog(ctx context.Context, item actiondomain.CatalogItem) error
	GetCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error)
	CreateProposal(ctx context.Context, in actiondomain.Proposal) (actiondomain.Proposal, error)
	GetProposalByID(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Proposal, error)
	GetProposalByIdempotencyKey(ctx context.Context, orgID uuid.UUID, idempotencyKey string) (actiondomain.Proposal, error)
	UpdateProposalStatus(ctx context.Context, orgID, proposalID uuid.UUID, status actiondomain.ProposalStatus) (actiondomain.Proposal, error)
	CreateExecution(ctx context.Context, in actiondomain.Execution) (actiondomain.Execution, error)
	CreateApproval(ctx context.Context, in actiondomain.Approval) (actiondomain.Approval, error)
	GetLatestApproval(ctx context.Context, orgID, proposalID uuid.UUID) (actiondomain.Approval, error)
}

type engine struct {
	repo        engineRepoPort
	emitter     EventEmitterPort
	tenant      TenantPort
	cfg         EngineConfig
	schemaCache *jsonschema.CompilerCache
}

func NewEngine(repo engineRepoPort, emitter EventEmitterPort, tenant TenantPort, cfg EngineConfig, schemaCache *jsonschema.CompilerCache) Engine {
	if cfg.DefaultMaxTTL <= 0 {
		cfg.DefaultMaxTTL = 1800
	}
	if strings.TrimSpace(cfg.ActionSchemaDir) == "" {
		cfg.ActionSchemaDir = filepath.Join("internal", "ops", "schemas", "actions")
	}
	if schemaCache == nil {
		schemaCache = jsonschema.NewCompilerCache()
	}
	return &engine{
		repo:        repo,
		emitter:     emitter,
		tenant:      tenant,
		cfg:         cfg,
		schemaCache: schemaCache,
	}
}

func (e *engine) DryRun(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error) {
	prepared, err := e.prepare(ctx, orgID, actor, req)
	if err != nil {
		return EngineResult{}, err
	}

	if prepared.existingReplay != nil {
		return EngineResult{
			Proposal:         *prepared.existingReplay,
			IdempotencyKey:   prepared.idempotencyKey,
			ScopeHash:        prepared.scopeHash,
			ParamsHash:       prepared.paramsHash,
			ApprovalRequired: prepared.existingReplay.ApprovalRequired,
			Replay:           true,
		}, nil
	}

	status := actiondomain.ProposalStatusDryRunOK
	if prepared.approvalRequired {
		status = actiondomain.ProposalStatusAwaitingApproval
	}
	proposal, err := e.repo.CreateProposal(ctx, actiondomain.Proposal{
		OrgID:            orgID,
		IncidentID:       req.IncidentID,
		ActionType:       prepared.actionType,
		Scope:            prepared.scopeCanonical,
		Params:           prepared.paramsCanonical,
		TTLSeconds:       req.TTLSeconds,
		EvidenceRefs:     req.EvidenceRefs,
		IdempotencyKey:   prepared.idempotencyKey,
		Status:           status,
		ApprovalRequired: prepared.approvalRequired,
		ProposedBy:       actor,
	})
	if err != nil {
		return EngineResult{}, err
	}

	execution, err := e.repo.CreateExecution(ctx, actiondomain.Execution{
		ProposalID: proposal.ID,
		OrgID:      orgID,
		Mode:       actiondomain.ExecutionModeDryRun,
		Status:     actiondomain.ExecutionStatusOK,
		Output: map[string]any{
			"scope_hash":      prepared.scopeHash,
			"params_hash":     prepared.paramsHash,
			"idempotency_key": prepared.idempotencyKey,
		},
		ExecutedBy: actor,
	})
	if err != nil {
		return EngineResult{}, err
	}

	e.emitActionProposed(ctx, orgID, actor, req.IncidentID, proposal, req)
	_ = e.emit(ctx, orgID, actor, req.IncidentID, &proposal.ID, "action.dry_run_ok", map[string]any{
		"proposal_id": proposal.ID.String(),
		"action_type": proposal.ActionType,
		"checks":      []string{"schema_valid", "ttl_valid", "scope_valid", "idempotency_valid"},
	})

	return EngineResult{
		Proposal:         proposal,
		Execution:        &execution,
		IdempotencyKey:   prepared.idempotencyKey,
		ScopeHash:        prepared.scopeHash,
		ParamsHash:       prepared.paramsHash,
		ApprovalRequired: proposal.ApprovalRequired,
		Replay:           false,
	}, nil
}

func (e *engine) Apply(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error) {
	var proposal actiondomain.Proposal
	var dryRunResult EngineResult
	var err error

	if req.ProposalID != nil {
		proposal, err = e.repo.GetProposalByID(ctx, orgID, *req.ProposalID)
		if err != nil {
			return EngineResult{}, err
		}
	} else {
		dryRunResult, err = e.DryRun(ctx, orgID, actor, req)
		if err != nil {
			return EngineResult{}, err
		}
		proposal = dryRunResult.Proposal
	}

	if proposal.ApprovalRequired && !req.ApprovalGranted {
		return EngineResult{}, types.NewHTTPError(409, types.ErrCodeApprovalRequired, "APPROVAL_REQUIRED")
	}
	if proposal.ApprovalRequired && req.ApprovalGranted && actor != nil {
		_, _ = e.repo.CreateApproval(ctx, actiondomain.Approval{
			ProposalID: proposal.ID,
			OrgID:      orgID,
			Approved:   true,
			Approver:   *actor,
			Comment:    req.ApprovalComment,
		})
	}

	execution, err := e.repo.CreateExecution(ctx, actiondomain.Execution{
		ProposalID: proposal.ID,
		OrgID:      orgID,
		Mode:       actiondomain.ExecutionModeApply,
		Status:     actiondomain.ExecutionStatusOK,
		Output: map[string]any{
			"applied": true,
		},
		ExecutedBy: actor,
	})
	if err != nil {
		return EngineResult{}, err
	}

	updated, err := e.repo.UpdateProposalStatus(ctx, orgID, proposal.ID, actiondomain.ProposalStatusApplied)
	if err != nil {
		return EngineResult{}, err
	}
	_ = e.emit(ctx, orgID, actor, updated.IncidentID, &updated.ID, "action.applied", map[string]any{
		"proposal_id":   updated.ID.String(),
		"action_id":     execution.ID.String(),
		"action_type":   updated.ActionType,
		"scope":         updated.Scope,
		"ttl_seconds":   updated.TTLSeconds,
		"applied_by":    actor,
	})

	idemKey := dryRunResult.IdempotencyKey
	if idemKey == "" {
		idemKey = proposal.IdempotencyKey
	}
	return EngineResult{
		Proposal:         updated,
		Execution:        &execution,
		IdempotencyKey:   idemKey,
		ApprovalRequired: updated.ApprovalRequired,
		Replay:           dryRunResult.Replay,
	}, nil
}

func (e *engine) Rollback(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (EngineResult, error) {
	if req.ProposalID == nil {
		return EngineResult{}, types.NewHTTPError(400, types.ErrCodeValidation, "proposal_id is required")
	}
	proposal, err := e.repo.GetProposalByID(ctx, orgID, *req.ProposalID)
	if err != nil {
		return EngineResult{}, err
	}

	execution, err := e.repo.CreateExecution(ctx, actiondomain.Execution{
		ProposalID: proposal.ID,
		OrgID:      orgID,
		Mode:       actiondomain.ExecutionModeRollback,
		Status:     actiondomain.ExecutionStatusOK,
		Output: map[string]any{
			"rolled_back": true,
		},
		ExecutedBy: actor,
	})
	if err != nil {
		return EngineResult{}, err
	}

	updated, err := e.repo.UpdateProposalStatus(ctx, orgID, proposal.ID, actiondomain.ProposalStatusRolledBack)
	if err != nil {
		return EngineResult{}, err
	}
	_ = e.emit(ctx, orgID, actor, updated.IncidentID, &updated.ID, "action.rolled_back", map[string]any{
		"action_id":   execution.ID.String(),
		"action_type": updated.ActionType,
		"reason":      "manual_or_automatic_rollback",
	})
	return EngineResult{
		Proposal:         updated,
		Execution:        &execution,
		IdempotencyKey:   updated.IdempotencyKey,
		ApprovalRequired: updated.ApprovalRequired,
	}, nil
}

type preparedInput struct {
	actionType       string
	scopeCanonical   map[string]any
	paramsCanonical  map[string]any
	scopeHash        string
	paramsHash       string
	idempotencyKey   string
	approvalRequired bool
	existingReplay   *actiondomain.Proposal
}

func (e *engine) prepare(ctx context.Context, orgID uuid.UUID, actor *string, req EngineRequest) (preparedInput, error) {
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

	scopeCanonical, err := canonicalizeScope(req.Scope)
	if err != nil {
		return preparedInput{}, types.NewHTTPError(400, types.ErrCodeValidation, err.Error())
	}
	paramsCanonical := normalizeJSON(req.Params, true).(map[string]any)

	scopeHash, err := hashCanonical(scopeCanonical)
	if err != nil {
		return preparedInput{}, err
	}
	paramsHash, err := hashCanonical(paramsCanonical)
	if err != nil {
		return preparedInput{}, err
	}
	incidentPart := ""
	if req.IncidentID != nil {
		incidentPart = req.IncidentID.String()
	}
	idempotencyKey := hashString(incidentPart + "|" + actionType + "|" + scopeHash + "|" + paramsHash)

	if existing, err := e.repo.GetProposalByIdempotencyKey(ctx, orgID, idempotencyKey); err == nil {
		return preparedInput{
			actionType:      actionType,
			scopeCanonical:  scopeCanonical,
			paramsCanonical: paramsCanonical,
			scopeHash:       scopeHash,
			paramsHash:      paramsHash,
			idempotencyKey:  idempotencyKey,
			existingReplay:  &existing,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return preparedInput{}, err
	}

	catalog, err := e.ensureCatalog(ctx, actionType)
	if err != nil {
		return preparedInput{}, err
	}
	maxTTL := e.cfg.DefaultMaxTTL
	if catalog.MaxTTLSeconds > 0 {
		maxTTL = catalog.MaxTTLSeconds
	}
	if e.tenant != nil {
		if profile, err := e.tenant.GetProfile(ctx, orgID); err == nil && profile.MaxTTLSeconds > 0 && profile.MaxTTLSeconds < maxTTL {
			maxTTL = profile.MaxTTLSeconds
		}
	}
	if req.TTLSeconds > maxTTL {
		return preparedInput{}, types.NewHTTPError(400, types.ErrCodeValidation, fmt.Sprintf("ttl_seconds exceeds max allowed (%d)", maxTTL))
	}

	if err := e.validateActionSchema(ctx, catalog, req, scopeCanonical, paramsCanonical); err != nil {
		return preparedInput{}, err
	}
	approvalRequired := catalog.RequiresApproval || requiresApprovalByBlastRadius(actionType, scopeCanonical)
	_ = actor
	return preparedInput{
		actionType:       actionType,
		scopeCanonical:   scopeCanonical,
		paramsCanonical:  paramsCanonical,
		scopeHash:        scopeHash,
		paramsHash:       paramsHash,
		idempotencyKey:   idempotencyKey,
		approvalRequired: approvalRequired,
	}, nil
}

func (e *engine) ensureCatalog(ctx context.Context, actionType string) (actiondomain.CatalogItem, error) {
	item, err := e.repo.GetCatalog(ctx, actionType)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return actiondomain.CatalogItem{}, err
	}
	builtins := builtinCatalogDefaults()
	def, ok := builtins[actionType]
	if !ok {
		return actiondomain.CatalogItem{}, types.NewHTTPError(400, types.ErrCodeValidation, "unsupported action_type")
	}
	schemaPath := filepath.Join(e.cfg.ActionSchemaDir, def.schemaFile)
	raw, readErr := os.ReadFile(schemaPath)
	if readErr != nil {
		return actiondomain.CatalogItem{}, readErr
	}
	var schema map[string]any
	if unmarshalErr := json.Unmarshal(raw, &schema); unmarshalErr != nil {
		return actiondomain.CatalogItem{}, unmarshalErr
	}
	item = actiondomain.CatalogItem{
		ActionType:       actionType,
		Schema:           schema,
		RequiresApproval: def.requiresApproval,
		MaxTTLSeconds:    def.maxTTLSeconds,
		Enabled:          true,
	}
	_ = e.repo.UpsertCatalog(ctx, item)
	return item, nil
}

func (e *engine) validateActionSchema(ctx context.Context, catalog actiondomain.CatalogItem, req EngineRequest, scope map[string]any, params map[string]any) error {
	raw, err := json.Marshal(catalog.Schema)
	if err != nil {
		return types.NewHTTPError(400, types.ErrCodeValidation, "invalid catalog schema")
	}
	sch, err := e.schemaCache.Compile(ctx, "action-schema-"+catalog.ActionType, raw)
	if err != nil {
		return types.NewHTTPError(500, types.ErrCodeSchemaInvalid, "catalog schema compile failed")
	}
	candidate := map[string]any{
		"action_type":   req.ActionType,
		"scope":         scope,
		"ttl_seconds":   req.TTLSeconds,
		"params":        params,
	}
	if len(req.EvidenceRefs) > 0 {
		candidate["evidence_refs"] = req.EvidenceRefs
	}
	if err := jsonschema.Validate(sch, candidate); err != nil {
		return types.NewHTTPError(400, types.ErrCodeValidation, "action does not match catalog schema: "+err.Error())
	}
	return nil
}

type catalogDefaults struct {
	schemaFile       string
	requiresApproval bool
	maxTTLSeconds    int
}

func builtinCatalogDefaults() map[string]catalogDefaults {
	return map[string]catalogDefaults{
		"set_safe_mode": {
			schemaFile:       "set_safe_mode_v1.json",
			requiresApproval: false,
			maxTTLSeconds:    1800,
		},
		"pause_tool": {
			schemaFile:       "pause_tool_v1.json",
			requiresApproval: false,
			maxTTLSeconds:    1800,
		},
		"quarantine_tenant": {
			schemaFile:       "quarantine_tenant_v1.json",
			requiresApproval: true,
			maxTTLSeconds:    1200,
		},
		"set_rate_limit": {
			schemaFile:       "set_rate_limit_v1.json",
			requiresApproval: false,
			maxTTLSeconds:    1800,
		},
		"rollback_last_mitigation": {
			schemaFile:       "rollback_last_mitigation_v1.json",
			requiresApproval: false,
			maxTTLSeconds:    900,
		},
		"export_audit": {
			schemaFile:       "export_audit_v1.json",
			requiresApproval: false,
			maxTTLSeconds:    1800,
		},
	}
}

func requiresApprovalByBlastRadius(actionType string, scope map[string]any) bool {
	level := strings.ToLower(strings.TrimSpace(asString(scope["level"])))
	if level == "global" {
		return true
	}
	if actionType == "quarantine_tenant" {
		return true
	}
	return false
}

func canonicalizeScope(scope map[string]any) (map[string]any, error) {
	level := strings.ToLower(strings.TrimSpace(asString(scope["level"])))
	switch level {
	case "global":
		if err := validateScopeKeys(scope, "level"); err != nil {
			return nil, err
		}
		return map[string]any{"level": "global"}, nil
	case "org":
		if err := validateScopeKeys(scope, "level", "org_id"); err != nil {
			return nil, err
		}
		orgID := strings.TrimSpace(asString(scope["org_id"]))
		if orgID == "" {
			return nil, fmt.Errorf("scope.org_id is required for level=org")
		}
		return map[string]any{"level": "org", "org_id": orgID}, nil
	case "tool":
		if err := validateScopeKeys(scope, "level", "org_id", "tool_id"); err != nil {
			return nil, err
		}
		orgID := strings.TrimSpace(asString(scope["org_id"]))
		toolID := strings.TrimSpace(asString(scope["tool_id"]))
		if orgID == "" || toolID == "" {
			return nil, fmt.Errorf("scope.org_id and scope.tool_id are required for level=tool")
		}
		return map[string]any{"level": "tool", "org_id": orgID, "tool_id": toolID}, nil
	case "actor":
		if err := validateScopeKeys(scope, "level", "org_id", "actor_id"); err != nil {
			return nil, err
		}
		orgID := strings.TrimSpace(asString(scope["org_id"]))
		actorID := strings.TrimSpace(asString(scope["actor_id"]))
		if orgID == "" || actorID == "" {
			return nil, fmt.Errorf("scope.org_id and scope.actor_id are required for level=actor")
		}
		return map[string]any{"level": "actor", "org_id": orgID, "actor_id": actorID}, nil
	default:
		return nil, fmt.Errorf("scope.level must be one of global|org|tool|actor")
	}
}

func validateScopeKeys(scope map[string]any, allowed ...string) error {
	allowedSet := map[string]struct{}{}
	for _, k := range allowed {
		allowedSet[k] = struct{}{}
	}
	for k, v := range scope {
		if _, ok := allowedSet[k]; !ok {
			return fmt.Errorf("scope contains unsupported key: %s", k)
		}
		if v == nil {
			return fmt.Errorf("scope.%s must not be null", k)
		}
	}
	return nil
}

func normalizeJSON(v any, excludeTTL bool) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if excludeTTL && k == "ttl_seconds" {
				continue
			}
			if t[k] == nil {
				continue
			}
			out[k] = normalizeJSON(t[k], excludeTTL)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, it := range t {
			if it == nil {
				continue
			}
			out = append(out, normalizeJSON(it, excludeTTL))
		}
		return out
	default:
		return v
	}
}

func hashCanonical(v any) (string, error) {
	raw, err := json.Marshal(normalizeJSON(v, false))
	if err != nil {
		return "", err
	}
	return hashString(string(raw)), nil
}

func hashString(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func actorTypeFromActor(actor *string) string {
	if actor == nil || strings.TrimSpace(*actor) == "" {
		return "system"
	}
	return "human"
}

func (e *engine) emitActionProposed(ctx context.Context, orgID uuid.UUID, actor *string, incidentID *uuid.UUID, proposal actiondomain.Proposal, req EngineRequest) {
	_ = e.emit(ctx, orgID, actor, incidentID, &proposal.ID, "action.proposed", map[string]any{
		"proposal_id":       proposal.ID.String(),
		"incident_id":       uuidToString(incidentID),
		"action_type":       proposal.ActionType,
		"scope":             proposal.Scope,
		"params":            proposal.Params,
		"ttl_seconds":       proposal.TTLSeconds,
		"approval_required": proposal.ApprovalRequired,
		"evidence_refs":     req.EvidenceRefs,
	})
}

func (e *engine) emit(ctx context.Context, orgID uuid.UUID, actor *string, incidentID *uuid.UUID, proposalID *uuid.UUID, eventType string, payload map[string]any) error {
	if e.emitter == nil {
		return nil
	}
	corr := opsdomain.Correlation{}
	if incidentID != nil {
		id := incidentID.String()
		corr.IncidentID = &id
	}
	if proposalID != nil {
		id := proposalID.String()
		corr.ActionID = &id
	}
	_, err := e.emitter.Emit(ctx, opseventstore.EmitInput{
		EventType: eventType,
		Version:   1,
		OrgID:     orgID,
		Correlation: corr,
		Actor: opsdomain.Actor{
			ActorID:   actor,
			ActorType: actorTypeFromActor(actor),
		},
		Source:  "ops.action_engine",
		Payload: payload,
	})
	return err
}

func uuidToString(v *uuid.UUID) string {
	if v == nil {
		return ""
	}
	return v.String()
}
