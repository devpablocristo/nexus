package policies

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	policydomain "nexus/v2/control-plane/internal/policies/usecases/domain"
)

type CreateRequest struct {
	ActionType         string
	ResourceType       string
	Effect             string
	Priority           int
	Expression         string
	Reason             string
	RequireApproval    bool
	ApprovalTTLSeconds int
	Enabled            *bool
}

type ListRequest struct {
	ActionType   string
	ResourceType string
	Archived     *bool
}

type PolicyPatch struct {
	ActionType         *string
	ResourceType       *string
	Effect             *string
	Priority           *int
	Expression         *string
	Reason             *string
	RequireApproval    *bool
	ApprovalTTLSeconds *int
	Enabled            *bool
}

type CELValidator interface {
	Validate(expression string) error
}

type httpError struct {
	Status  int
	Code    string
	Message string
}

func (e httpError) Error() string { return e.Message }

func newHTTPError(status int, code, message string) error {
	return httpError{Status: status, Code: code, Message: message}
}

type Usecases struct {
	repo      Repository
	validator CELValidator
}

func NewUsecases(repo Repository, validator CELValidator) *Usecases {
	return &Usecases{repo: repo, validator: validator}
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (policydomain.Policy, error) {
	if strings.TrimSpace(req.ActionType) == "" {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "action_type required")
	}
	if strings.TrimSpace(req.ResourceType) == "" {
		return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_type required")
	}
	if err := validateEffect(req.Effect); err != nil {
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
		ActionType:         strings.TrimSpace(req.ActionType),
		ResourceType:       strings.TrimSpace(req.ResourceType),
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
	if item.ArchivedAt != nil {
		return policydomain.Policy{}, newHTTPError(http.StatusConflict, "ARCHIVED", "archived policies cannot be modified")
	}

	if patch.ActionType != nil {
		if strings.TrimSpace(*patch.ActionType) == "" {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "action_type required")
		}
		item.ActionType = strings.TrimSpace(*patch.ActionType)
	}
	if patch.ResourceType != nil {
		if strings.TrimSpace(*patch.ResourceType) == "" {
			return policydomain.Policy{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "resource_type required")
		}
		item.ResourceType = strings.TrimSpace(*patch.ResourceType)
	}
	if patch.Effect != nil {
		if err := validateEffect(*patch.Effect); err != nil {
			return policydomain.Policy{}, err
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

func normalizeApprovalTTL(requireApproval bool, ttl int) int {
	if !requireApproval {
		return 0
	}
	if ttl <= 0 {
		return 3600
	}
	return ttl
}

func validateEffect(effect string) error {
	if effect != string(policydomain.EffectAllow) && effect != string(policydomain.EffectDeny) {
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "effect must be allow or deny")
	}
	return nil
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "policy not found")
	}
	if errors.Is(err, ErrArchived) {
		return newHTTPError(http.StatusConflict, "ARCHIVED", "policy is archived")
	}
	if errors.Is(err, ErrAlreadyArchived) {
		return newHTTPError(http.StatusConflict, "ALREADY_ARCHIVED", "policy already archived")
	}
	if errors.Is(err, ErrNotArchived) {
		return newHTTPError(http.StatusConflict, "NOT_ARCHIVED", "policy is not archived")
	}
	return err
}
