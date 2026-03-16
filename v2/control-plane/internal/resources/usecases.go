package resources

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	resourcedomain "nexus/v2/control-plane/internal/resources/usecases/domain"
)

type CreateRequest struct {
	Type        resourcedomain.ResourceType
	Name        string
	Environment string
	Chain       string
	Labels      map[string]string
	Criticality resourcedomain.Criticality
	IsCanary    bool
}

type UpdateRequest struct {
	Type        *resourcedomain.ResourceType
	Name        *string
	Environment *string
	Chain       *string
	Labels      map[string]string
	Criticality *resourcedomain.Criticality
	IsCanary    *bool
}

type ListRequest struct {
	Type        string
	Environment string
	Archived    *bool
	Limit       int
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
	return httpError{Status: status, Code: code, Message: message}
}

type Usecases struct {
	repo Repository
}

func NewUsecases(repo Repository) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) Create(ctx context.Context, req CreateRequest) (resourcedomain.ProtectedResource, error) {
	normalized, err := normalizeCreate(req)
	if err != nil {
		return resourcedomain.ProtectedResource{}, err
	}
	return u.repo.Create(ctx, normalized)
}

func (u *Usecases) List(ctx context.Context, req ListRequest) ([]resourcedomain.ProtectedResource, error) {
	archived := req.Archived
	if archived == nil {
		defaultArchived := false
		archived = &defaultArchived
	}

	filters := ListFilters{
		Type:        strings.TrimSpace(req.Type),
		Environment: strings.TrimSpace(req.Environment),
		Archived:    archived,
		Limit:       req.Limit,
	}
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Type != "" {
		if err := validateType(resourcedomain.ResourceType(filters.Type)); err != nil {
			return nil, err
		}
	}
	return u.repo.List(ctx, filters)
}

func (u *Usecases) GetByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error) {
	item, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return resourcedomain.ProtectedResource{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) UpdateByID(ctx context.Context, id uuid.UUID, req UpdateRequest) (resourcedomain.ProtectedResource, error) {
	current, err := u.GetByID(ctx, id)
	if err != nil {
		return resourcedomain.ProtectedResource{}, err
	}

	if req.Type != nil {
		if err := validateType(*req.Type); err != nil {
			return resourcedomain.ProtectedResource{}, err
		}
		current.Type = *req.Type
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "name required")
		}
		current.Name = name
	}
	if req.Environment != nil {
		environment := strings.TrimSpace(*req.Environment)
		if environment == "" {
			return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "environment required")
		}
		current.Environment = environment
	}
	if req.Chain != nil {
		chain := strings.TrimSpace(*req.Chain)
		if chain == "" {
			return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "chain required")
		}
		current.Chain = chain
	}
	if req.Labels != nil {
		current.Labels = cloneLabels(req.Labels)
	}
	if req.Criticality != nil {
		if err := validateCriticality(*req.Criticality); err != nil {
			return resourcedomain.ProtectedResource{}, err
		}
		current.Criticality = *req.Criticality
	}
	if req.IsCanary != nil {
		current.IsCanary = *req.IsCanary
		current.Labels = applyCanaryLabel(current.Labels, current.IsCanary)
	}

	updated, err := u.repo.Update(ctx, current)
	if err != nil {
		return resourcedomain.ProtectedResource{}, mapRepoErr(err)
	}
	return updated, nil
}

func (u *Usecases) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return mapRepoErr(u.repo.Delete(ctx, id))
}

func (u *Usecases) ArchiveByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error) {
	item, err := u.repo.Archive(ctx, id, nowUTC())
	if err != nil {
		return resourcedomain.ProtectedResource{}, mapRepoErr(err)
	}
	return item, nil
}

func (u *Usecases) RestoreByID(ctx context.Context, id uuid.UUID) (resourcedomain.ProtectedResource, error) {
	item, err := u.repo.Restore(ctx, id, nowUTC())
	if err != nil {
		return resourcedomain.ProtectedResource{}, mapRepoErr(err)
	}
	return item, nil
}

func normalizeCreate(req CreateRequest) (resourcedomain.ProtectedResource, error) {
	if err := validateType(req.Type); err != nil {
		return resourcedomain.ProtectedResource{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "name required")
	}
	environment := strings.TrimSpace(req.Environment)
	if environment == "" {
		return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "environment required")
	}
	chain := strings.TrimSpace(req.Chain)
	if chain == "" {
		return resourcedomain.ProtectedResource{}, newHTTPError(http.StatusBadRequest, "VALIDATION", "chain required")
	}
	if err := validateCriticality(req.Criticality); err != nil {
		return resourcedomain.ProtectedResource{}, err
	}

	return resourcedomain.ProtectedResource{
		Type:        req.Type,
		Name:        name,
		Environment: environment,
		Chain:       chain,
		Labels:      applyCanaryLabel(cloneLabels(req.Labels), req.IsCanary),
		Criticality: req.Criticality,
		IsCanary:    req.IsCanary,
	}, nil
}

func applyCanaryLabel(labels map[string]string, isCanary bool) map[string]string {
	out := cloneLabels(labels)
	if out == nil {
		out = map[string]string{}
	}
	if isCanary {
		out["_nexus_trap"] = "true"
		return out
	}
	delete(out, "_nexus_trap")
	return out
}

func validateType(value resourcedomain.ResourceType) error {
	switch value {
	case resourcedomain.ResourceTypeWallet, resourcedomain.ResourceTypeTreasury, resourcedomain.ResourceTypeVault:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported type")
	}
}

func validateCriticality(value resourcedomain.Criticality) error {
	switch value {
	case resourcedomain.CriticalityLow, resourcedomain.CriticalityMedium, resourcedomain.CriticalityHigh, resourcedomain.CriticalityCritical:
		return nil
	default:
		return newHTTPError(http.StatusBadRequest, "VALIDATION", "unsupported criticality")
	}
}

func mapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return newHTTPError(http.StatusNotFound, "NOT_FOUND", "resource not found")
	}
	if errors.Is(err, ErrArchived) {
		return newHTTPError(http.StatusConflict, "ARCHIVED", "resource is archived")
	}
	if errors.Is(err, ErrAlreadyArchived) {
		return newHTTPError(http.StatusConflict, "ALREADY_ARCHIVED", "resource already archived")
	}
	if errors.Is(err, ErrNotArchived) {
		return newHTTPError(http.StatusConflict, "NOT_ARCHIVED", "resource is not archived")
	}
	return err
}
