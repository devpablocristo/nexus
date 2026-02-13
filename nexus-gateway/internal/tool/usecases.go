package tool

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	tooldomain "nexus-gateway/internal/tool/usecases/domain"
	"nexus-gateway/pkg/types"
	"nexus-gateway/pkg/validations/jsonschema"
)

type RepositoryPort interface {
	Create(ctx context.Context, orgID uuid.UUID, t tooldomain.Tool) (tooldomain.Tool, error)
	List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	UpdateByName(ctx context.Context, orgID uuid.UUID, name string, patch ToolPatch) (tooldomain.Tool, error)
}

type Service interface {
	Create(ctx context.Context, orgID uuid.UUID, req CreateRequest) (tooldomain.Tool, error)
	List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error)
	GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error)
	UpdateByName(ctx context.Context, orgID uuid.UUID, name string, patch ToolPatch) (tooldomain.Tool, error)
}

type CreateRequest struct {
	Name           string
	Kind           string
	Description    *string
	Method         string
	URL            string
	InputSchema    map[string]any
	OutputSchema   map[string]any
	ActionType     string
	Classification string
	Sensitivity    string
	RiskLevel      int
	Enabled        bool
}

type ToolPatch struct {
	Description    **string
	Method         *string
	URL            *string
	InputSchema    *map[string]any
	OutputSchema   *map[string]any
	ActionType     *string
	Classification *string
	Sensitivity    *string
	RiskLevel      *int
	Enabled        *bool
}

type ServiceImpl struct {
	repo  RepositoryPort
	cache *jsonschema.CompilerCache
}

func NewService(repo RepositoryPort, cache *jsonschema.CompilerCache) *ServiceImpl {
	return &ServiceImpl{repo: repo, cache: cache}
}

func (s *ServiceImpl) Create(ctx context.Context, orgID uuid.UUID, req CreateRequest) (tooldomain.Tool, error) {
	if req.Name == "" {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "name required")
	}
	if req.Kind != string(tooldomain.ToolKindHTTP) {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "kind must be http")
	}
	if req.Method == "" || req.URL == "" {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "method and url required")
	}
	if req.ActionType != string(tooldomain.ActionRead) && req.ActionType != string(tooldomain.ActionWrite) {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "action_type must be read|write")
	}
	if req.Classification == "" {
		req.Classification = "internal"
	}
	if req.Classification != "internal" && req.Classification != "external" {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeToolClassInvalid, "classification must be internal|external")
	}
	if req.Sensitivity == "" {
		req.Sensitivity = "low"
	}
	if req.Sensitivity != "low" && req.Sensitivity != "medium" && req.Sensitivity != "high" {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeToolSensitivity, "sensitivity must be low|medium|high")
	}
	if req.RiskLevel < 1 || req.RiskLevel > 5 {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "risk_level must be 1..5")
	}
	inSchemaBytes, err := json.Marshal(req.InputSchema)
	if err != nil {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "input_schema invalid")
	}
	if _, err := s.cache.Compile(ctx, orgID.String()+":"+req.Name+":in", inSchemaBytes); err != nil {
		return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeSchemaInvalid, "input_schema is not a valid JSON schema")
	}

	var outSchemaBytes []byte
	if req.OutputSchema != nil {
		outSchemaBytes, err = json.Marshal(req.OutputSchema)
		if err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "output_schema invalid")
		}
		if len(outSchemaBytes) > 0 {
			if _, err := s.cache.Compile(ctx, orgID.String()+":"+req.Name+":out", outSchemaBytes); err != nil {
				return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeSchemaInvalid, "output_schema is not a valid JSON schema")
			}
		}
	}

	t := tooldomain.Tool{
		OrgID:            orgID,
		Name:             req.Name,
		Kind:             tooldomain.ToolKind(req.Kind),
		Description:      req.Description,
		Method:           req.Method,
		URL:              req.URL,
		InputSchemaJSON:  inSchemaBytes,
		OutputSchemaJSON: outSchemaBytes,
		ActionType:       tooldomain.ActionType(req.ActionType),
		Classification:   req.Classification,
		Sensitivity:      req.Sensitivity,
		RiskLevel:        req.RiskLevel,
		Enabled:          req.Enabled,
	}
	created, err := s.repo.Create(ctx, orgID, t)
	if err != nil {
		var he types.HTTPError
		if errors.As(err, &he) {
			return tooldomain.Tool{}, he
		}
		return tooldomain.Tool{}, err
	}
	return created, nil
}

func (s *ServiceImpl) List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error) {
	return s.repo.List(ctx, orgID)
}

func (s *ServiceImpl) GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error) {
	return s.repo.GetByName(ctx, orgID, name)
}

func (s *ServiceImpl) UpdateByName(ctx context.Context, orgID uuid.UUID, name string, patch ToolPatch) (tooldomain.Tool, error) {
	if patch.InputSchema != nil {
		b, err := json.Marshal(*patch.InputSchema)
		if err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "input_schema invalid")
		}
		if _, err := s.cache.Compile(ctx, orgID.String()+":"+name+":in", b); err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeSchemaInvalid, "input_schema is not a valid JSON schema")
		}
	}
	if patch.OutputSchema != nil {
		b, err := json.Marshal(*patch.OutputSchema)
		if err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "output_schema invalid")
		}
		if _, err := s.cache.Compile(ctx, orgID.String()+":"+name+":out", b); err != nil {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeSchemaInvalid, "output_schema is not a valid JSON schema")
		}
	}
	if patch.Classification != nil {
		if *patch.Classification != "internal" && *patch.Classification != "external" {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeToolClassInvalid, "classification must be internal|external")
		}
	}
	if patch.Sensitivity != nil {
		if *patch.Sensitivity != "low" && *patch.Sensitivity != "medium" && *patch.Sensitivity != "high" {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeToolSensitivity, "sensitivity must be low|medium|high")
		}
	}
	return s.repo.UpdateByName(ctx, orgID, name, patch)
}
