package policy

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	policydomain "nexus/v2/data-plane/internal/policy/usecases/domain"
	"nexus/v2/data-plane/internal/tool"
)

type PolicyRepositoryPort interface {
	Create(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error)
	List(ctx context.Context, filters ListFilters) ([]policydomain.Policy, error)
	GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	Save(ctx context.Context, item policydomain.Policy) (policydomain.Policy, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	ArchiveByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
	RestoreByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error)
}

type ToolLookupPort interface {
	GetByName(ctx context.Context, name string) (tool.Definition, error)
}

type CELValidator interface {
	Validate(expression string) error
}

type CreateRequest struct {
	ToolName           string
	Effect             string
	Priority           int
	Expression         string
	Reason             string
	RequireApproval    bool
	ApprovalTTLSeconds int
	Enabled            *bool
}

type ListRequest struct {
	ToolName        string
	IncludeArchived bool
}

type PolicyPatch struct {
	ToolName           *string
	Effect             *string
	Priority           *int
	Expression         *string
	Reason             *string
	RequireApproval    *bool
	ApprovalTTLSeconds *int
	Enabled            *bool
}

type Usecases struct {
	repo      PolicyRepositoryPort
	toolLook  ToolLookupPort
	validator CELValidator
}

func NewUsecases(repo PolicyRepositoryPort, toolLook ToolLookupPort, validator CELValidator) *Usecases {
	return &Usecases{
		repo:      repo,
		toolLook:  toolLook,
		validator: validator,
	}
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (policydomain.Policy, error) {
	if req.ToolName == "" {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "tool_name required")
	}
	if req.Effect != string(policydomain.EffectAllow) && req.Effect != string(policydomain.EffectDeny) {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "effect must be allow or deny")
	}
	if err := u.ensureToolExists(ctx, req.ToolName); err != nil {
		return policydomain.Policy{}, err
	}
	if err := u.validator.Validate(req.Expression); err != nil {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "INVALID_EXPRESSION", err.Error())
	}
	if req.RequireApproval && req.Effect != string(policydomain.EffectAllow) {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "approval requires effect allow")
	}
	if req.ApprovalTTLSeconds < 0 {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "approval_ttl_seconds must be >= 0")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Priority == 0 {
		req.Priority = 100
	}

	return u.repo.Create(ctx, policydomain.Policy{
		ToolName:           req.ToolName,
		Effect:             policydomain.Effect(req.Effect),
		Priority:           req.Priority,
		Expression:         req.Expression,
		Reason:             req.Reason,
		RequireApproval:    req.RequireApproval,
		ApprovalTTLSeconds: normalizeApprovalTTL(req.RequireApproval, req.ApprovalTTLSeconds),
		Enabled:            enabled,
	})
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]policydomain.Policy, error) {
	return u.repo.List(ctx, ListFilters(req))
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return policydomain.Policy{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) UpdateByID(ctx context.Context, id uuid.UUID, patch PolicyPatch) (policydomain.Policy, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return policydomain.Policy{}, mapRepoErr(err)
	}
	if item.Archived {
		return policydomain.Policy{}, newHTTPError(http.StatusConflict, "ARCHIVED", "archived policies cannot be modified")
	}

	if patch.ToolName != nil {
		if *patch.ToolName == "" {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "tool_name required")
		}
		if err := u.ensureToolExists(ctx, *patch.ToolName); err != nil {
			return policydomain.Policy{}, err
		}
		item.ToolName = *patch.ToolName
	}
	if patch.Effect != nil {
		if *patch.Effect != string(policydomain.EffectAllow) && *patch.Effect != string(policydomain.EffectDeny) {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "effect must be allow or deny")
		}
		item.Effect = policydomain.Effect(*patch.Effect)
	}
	if patch.Priority != nil {
		item.Priority = *patch.Priority
	}
	if patch.Expression != nil {
		if err := u.validator.Validate(*patch.Expression); err != nil {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "INVALID_EXPRESSION", err.Error())
		}
		item.Expression = *patch.Expression
	}
	if patch.Reason != nil {
		item.Reason = *patch.Reason
	}
	if patch.RequireApproval != nil {
		item.RequireApproval = *patch.RequireApproval
	}
	if patch.ApprovalTTLSeconds != nil {
		if *patch.ApprovalTTLSeconds < 0 {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "approval_ttl_seconds must be >= 0")
		}
		item.ApprovalTTLSeconds = *patch.ApprovalTTLSeconds
	}
	if patch.Enabled != nil {
		item.Enabled = *patch.Enabled
	}
	if item.RequireApproval && item.Effect != policydomain.EffectAllow {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "approval requires effect allow")
	}
	item.ApprovalTTLSeconds = normalizeApprovalTTL(item.RequireApproval, item.ApprovalTTLSeconds)

	return u.repo.Save(ctx, item)
}

func normalizeApprovalTTL(requireApproval bool, ttl int) int {
	if !requireApproval {
		return 0
	}
	if ttl <= 0 {
		return 3600
	}
	return ttl
}

func (u *Usecases) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return mapRepoErr(u.repo.DeleteByID(ctx, id))
}

func (u *Usecases) ArchiveByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	item, err := u.repo.ArchiveByID(ctx, id)
	if err != nil {
		return policydomain.Policy{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) RestoreByID(ctx context.Context, id uuid.UUID) (policydomain.Policy, error) {
	item, err := u.repo.RestoreByID(ctx, id)
	if err != nil {
		return policydomain.Policy{}, mapRepoErr(err)
	}
	return item, nil
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string {
	return e.Message
}

func newHTTPError(status int, code, message string) error {
	return httpError{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "policy not found")
	}
	return err
}

func (u *Usecases) ensureToolExists(ctx context.Context, toolName string) error {
	if _, err := u.toolLook.GetByName(ctx, toolName); err != nil {
		if errors.Is(err, tool.ErrNotFound) {
			return newHTTPError(http.StatusBadRequest, "VALIDATION", "tool not found")
		}
		return err
	}
	return nil
}
