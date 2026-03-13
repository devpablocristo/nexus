package actions

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	actiondomain "control-plane/internal/actions/usecases/domain"
	eventdomain "control-plane/internal/events/usecases/domain"
	"nexus/pkg/types"
)

type RepositoryPort interface {
	Create(ctx context.Context, a actiondomain.Action) (actiondomain.Action, error)
	GetByID(ctx context.Context, orgID, id uuid.UUID) (actiondomain.Action, error)
	List(ctx context.Context, orgID uuid.UUID, status, actionType string, limit int) ([]actiondomain.Action, error)
	UpdateStatus(ctx context.Context, orgID, id uuid.UUID, status actiondomain.Status, rolledBackBy *string, rolledBackAt *time.Time) (actiondomain.Action, error)
	ListExpiredCandidates(ctx context.Context, now time.Time, limit int) ([]actiondomain.Action, error)
	ListActiveForRun(ctx context.Context, orgID uuid.UUID, toolName string, now time.Time) ([]actiondomain.Action, error)
}

type EventSink interface {
	Append(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) (eventdomain.Event, error)
}

type MeteringPort interface {
	Increment(ctx context.Context, orgID uuid.UUID, counter string) error
}

type ApplyRequest struct {
	ScopeType    string
	ScopeID      *string
	ActionType   string
	Params       map[string]any
	TTLSeconds   int
	EvidenceRefs []string
}

type ListQuery struct {
	Status     string
	ActionType string
	Limit      int
}

type Usecases struct {
	repo     RepositoryPort
	events   EventSink
	metering MeteringPort
}

func NewUsecases(repo RepositoryPort, events EventSink, metering MeteringPort) *Usecases {
	return &Usecases{repo: repo, events: events, metering: metering}
}

func (u *Usecases) Apply(ctx context.Context, orgID uuid.UUID, actor *string, req ApplyRequest) (actiondomain.Action, error) {
	scopeType := actiondomain.ScopeType(req.ScopeType)
	switch scopeType {
	case actiondomain.ScopeTenant, actiondomain.ScopeTool, actiondomain.ScopeAgent, actiondomain.ScopeGlobal:
	default:
		return actiondomain.Action{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid scope_type")
	}
	actionType := actiondomain.ActionType(req.ActionType)
	switch actionType {
	case actiondomain.ActionThrottleTenantRPM, actiondomain.ActionThrottleToolRPM, actiondomain.ActionQuarantineTool, actiondomain.ActionDisableTool:
	default:
		return actiondomain.Action{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid action_type")
	}
	if scopeType == actiondomain.ScopeTool && (req.ScopeID == nil || *req.ScopeID == "") {
		return actiondomain.Action{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "scope_id required for tool scope")
	}
	if req.TTLSeconds < 0 {
		return actiondomain.Action{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "ttl_seconds must be >= 0")
	}
	if req.Params == nil {
		req.Params = map[string]any{}
	}
	created, err := u.repo.Create(ctx, actiondomain.Action{
		OrgID:        orgID,
		ScopeType:    scopeType,
		ScopeID:      req.ScopeID,
		ActionType:   actionType,
		Params:       req.Params,
		TTLSeconds:   req.TTLSeconds,
		Status:       actiondomain.StatusActive,
		EvidenceRefs: req.EvidenceRefs,
		CreatedBy:    actor,
	})
	if err != nil {
		return actiondomain.Action{}, err
	}
	if u.metering != nil {
		_ = u.metering.Increment(ctx, orgID, "actions_executed")
	}
	if u.events != nil {
		_, _ = u.events.Append(ctx, orgID, "action.applied", map[string]any{
			"action_id":      created.ID.String(),
			"scope_type":     string(created.ScopeType),
			"scope_id":       created.ScopeID,
			"action_type":    string(created.ActionType),
			"ttl_seconds":    created.TTLSeconds,
			"evidence_refs":  created.EvidenceRefs,
			"created_by":     created.CreatedBy,
			"created_at":     created.CreatedAt.UTC().Format(time.RFC3339),
			"effective_state": string(created.Status),
		})
	}
	return created, nil
}

func (u *Usecases) Rollback(ctx context.Context, orgID, id uuid.UUID, actor *string) (actiondomain.Action, error) {
	a, err := u.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if a.Status != actiondomain.StatusActive {
		return actiondomain.Action{}, types.NewHTTPError(http.StatusConflict, types.ErrCodeValidation, "action is not active")
	}
	now := time.Now().UTC()
	updated, err := u.repo.UpdateStatus(ctx, orgID, id, actiondomain.StatusRolledBack, actor, &now)
	if err != nil {
		return actiondomain.Action{}, err
	}
	if u.events != nil {
		_, _ = u.events.Append(ctx, orgID, "action.rolled_back", map[string]any{
			"action_id":   updated.ID.String(),
			"action_type": string(updated.ActionType),
			"rolled_back_by": actor,
			"rolled_back_at": now.Format(time.RFC3339),
		})
	}
	return updated, nil
}

func (u *Usecases) List(ctx context.Context, orgID uuid.UUID, q ListQuery) ([]actiondomain.Action, error) {
	return u.repo.List(ctx, orgID, q.Status, q.ActionType, q.Limit)
}

func (u *Usecases) ExpireDue(ctx context.Context, now time.Time, batch int) (int, error) {
	candidates, err := u.repo.ListExpiredCandidates(ctx, now, batch)
	if err != nil {
		return 0, err
	}
	expired := 0
	for _, a := range candidates {
		updated, err := u.repo.UpdateStatus(ctx, a.OrgID, a.ID, actiondomain.StatusExpired, nil, nil)
		if err != nil {
			continue
		}
		expired++
		if u.events != nil {
			_, _ = u.events.Append(ctx, updated.OrgID, "action.expired", map[string]any{
				"action_id":   updated.ID.String(),
				"action_type": string(updated.ActionType),
				"expired_at":  now.UTC().Format(time.RFC3339),
			})
		}
	}
	return expired, nil
}

func (u *Usecases) ResolveRuntimeOverrides(ctx context.Context, orgID uuid.UUID, toolName string) (actiondomain.RuntimeOverrides, error) {
	items, err := u.repo.ListActiveForRun(ctx, orgID, toolName, time.Now().UTC())
	if err != nil {
		return actiondomain.RuntimeOverrides{}, err
	}
	over := actiondomain.RuntimeOverrides{}
	for _, item := range items {
		over.ActiveActionIDs = append(over.ActiveActionIDs, item.ID.String())
		over.AppliedActionTypes = append(over.AppliedActionTypes, string(item.ActionType))
		switch item.ActionType {
		case actiondomain.ActionDisableTool, actiondomain.ActionQuarantineTool:
			over.Deny = true
			over.DenyReason = fmt.Sprintf("blocked by action %s", item.ActionType)
		case actiondomain.ActionThrottleTenantRPM:
			if v := intFromParams(item.Params, "per_minute"); v > 0 {
				over.TenantRPMOverride = minIntPtr(over.TenantRPMOverride, v)
			}
		case actiondomain.ActionThrottleToolRPM:
			if v := intFromParams(item.Params, "per_minute"); v > 0 {
				over.ToolRPMOverride = minIntPtr(over.ToolRPMOverride, v)
			}
		}
	}
	return over, nil
}

func intFromParams(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	default:
		return 0
	}
}

func minIntPtr(cur *int, v int) *int {
	if cur == nil {
		return &v
	}
	if v < *cur {
		return &v
	}
	return cur
}
