package policy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	policydomain "nexus-core/internal/policy/usecases/domain"
	tooldomain "nexus-core/internal/tool/usecases/domain"
	"nexus-core/pkg/types"
)

type PolicyRepositoryPort interface {
	Create(ctx context.Context, orgID uuid.UUID, p policydomain.Policy) (policydomain.Policy, error)
	ListByToolID(ctx context.Context, orgID, toolID uuid.UUID) ([]policydomain.Policy, error)
	GetByID(ctx context.Context, orgID, policyID uuid.UUID) (policydomain.Policy, error)
	Update(ctx context.Context, orgID uuid.UUID, policyID uuid.UUID, patch PolicyPatch) (policydomain.Policy, error)
}

type ToolLookupPort interface {
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
}

type Service interface {
	CreateForTool(ctx context.Context, orgID uuid.UUID, toolName string, req CreateRequest) (policydomain.Policy, error)
	ListForTool(ctx context.Context, orgID uuid.UUID, toolName string) ([]policydomain.Policy, error)
	UpdateByID(ctx context.Context, orgID uuid.UUID, policyID uuid.UUID, patch PolicyPatch) (policydomain.Policy, error)
}

type CreateRequest struct {
	Effect         string         `json:"effect"`
	Priority       int            `json:"priority"`
	Conditions     map[string]any `json:"conditions"`
	Limits         map[string]any `json:"limits"`
	ReasonTemplate string         `json:"reason_template"`
	Enabled        bool           `json:"enabled"`
}

type PolicyPatch struct {
	Effect         *string
	Priority       *int
	Conditions     *map[string]any
	Limits         *map[string]any
	ReasonTemplate *string
	Enabled        *bool
}

type service struct {
	repo     PolicyRepositoryPort
	toolLook ToolLookupPort
}

func NewService(repo PolicyRepositoryPort, toolLook ToolLookupPort) Service {
	return &service{repo: repo, toolLook: toolLook}
}

func (s *service) CreateForTool(ctx context.Context, orgID uuid.UUID, toolName string, req CreateRequest) (policydomain.Policy, error) {
	tool, err := s.toolLook.GetByName(ctx, orgID, toolName)
	if err != nil {
		return policydomain.Policy{}, err
	}
	if req.Effect != string(policydomain.EffectAllow) && req.Effect != string(policydomain.EffectDeny) {
		return policydomain.Policy{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "effect must be allow|deny")
	}
	if req.Priority == 0 {
		req.Priority = 100
	}
	condBytes, _ := json.Marshal(orEmptyObj(req.Conditions))
	limBytes, _ := json.Marshal(orEmptyObj(req.Limits))
	p := policydomain.Policy{
		OrgID:          orgID,
		ToolID:         tool.ID,
		Effect:         policydomain.Effect(req.Effect),
		Priority:       req.Priority,
		ConditionsJSON: condBytes,
		LimitsJSON:     limBytes,
		ReasonTemplate: req.ReasonTemplate,
		Enabled:        req.Enabled,
	}
	return s.repo.Create(ctx, orgID, p)
}

func (s *service) ListForTool(ctx context.Context, orgID uuid.UUID, toolName string) ([]policydomain.Policy, error) {
	tool, err := s.toolLook.GetByName(ctx, orgID, toolName)
	if err != nil {
		return nil, err
	}
	return s.repo.ListByToolID(ctx, orgID, tool.ID)
}

func (s *service) UpdateByID(ctx context.Context, orgID uuid.UUID, policyID uuid.UUID, patch PolicyPatch) (policydomain.Policy, error) {
	if patch.Effect != nil {
		if *patch.Effect != string(policydomain.EffectAllow) && *patch.Effect != string(policydomain.EffectDeny) {
			return policydomain.Policy{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "effect must be allow|deny")
		}
	}
	if patch.Conditions != nil {
		if _, err := json.Marshal(*patch.Conditions); err != nil {
			return policydomain.Policy{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "conditions invalid")
		}
	}
	if patch.Limits != nil {
		if _, err := json.Marshal(*patch.Limits); err != nil {
			return policydomain.Policy{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "limits invalid")
		}
	}
	_, err := s.repo.GetByID(ctx, orgID, policyID)
	if err != nil {
		var he types.HTTPError
		if errors.As(err, &he) {
			return policydomain.Policy{}, he
		}
		return policydomain.Policy{}, err
	}
	return s.repo.Update(ctx, orgID, policyID, patch)
}

func orEmptyObj(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
