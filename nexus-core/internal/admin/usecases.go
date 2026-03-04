package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	admindomain "nexus-core/internal/admin/usecases/domain"
	"nexus-core/internal/shared/authz"
	"nexus/pkg/types"
)

const (
	scopeConsoleRead  = "admin:console:read"
	scopeConsoleWrite = "admin:console:write"
)

type RepositoryPort interface {
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, bool, error)
	UpsertTenantSettings(ctx context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error)
	CreateAdminActivityEvent(ctx context.Context, ev admindomain.AdminActivityEvent) error
	ListAdminActivityEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]admindomain.AdminActivityEvent, error)
}

type Usecases struct {
	repo RepositoryPort
}

type BootstrapResponse struct {
	OrgID         uuid.UUID
	Actor         *string
	Role          *string
	Scopes        []string
	AuthMethod    string
	CanReadAdmin  bool
	CanWriteAdmin bool
	Settings      admindomain.TenantSettings
}

type UpsertTenantSettingsRequest struct {
	PlanCode   string
	HardLimits map[string]any
}

func NewUsecases(repo RepositoryPort) *Usecases {
	return &Usecases{repo: repo}
}

func (u *Usecases) GetBootstrap(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, authMethod string) (BootstrapResponse, error) {
	canRead, canWrite := adminCapabilities(role, scopes)
	if !canRead {
		return BootstrapResponse{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console read permission required")
	}
	settings, err := u.getOrDefaultSettings(ctx, orgID)
	if err != nil {
		return BootstrapResponse{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "admin.bootstrap.read",
		ResourceType: "admin_console",
		Payload: map[string]any{
			"auth_method": authMethod,
			"can_write":   canWrite,
		},
	})
	return BootstrapResponse{
		OrgID:         orgID,
		Actor:         actor,
		Role:          role,
		Scopes:        scopes,
		AuthMethod:    authMethod,
		CanReadAdmin:  canRead,
		CanWriteAdmin: canWrite,
		Settings:      settings,
	}, nil
}

func (u *Usecases) GetTenantSettings(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string) (admindomain.TenantSettings, error) {
	canRead, _ := adminCapabilities(role, scopes)
	if !canRead {
		return admindomain.TenantSettings{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console read permission required")
	}
	settings, err := u.getOrDefaultSettings(ctx, orgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "tenant_settings.read",
		ResourceType: "tenant_settings",
	})
	return settings, nil
}

func (u *Usecases) UpsertTenantSettings(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, req UpsertTenantSettingsRequest) (admindomain.TenantSettings, error) {
	_, canWrite := adminCapabilities(role, scopes)
	if !canWrite {
		return admindomain.TenantSettings{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console write permission required")
	}
	if req.PlanCode == "" {
		return admindomain.TenantSettings{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "plan_code required")
	}
	if req.HardLimits == nil {
		req.HardLimits = defaultHardLimits(req.PlanCode)
	}
	settings, err := u.repo.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   req.PlanCode,
		HardLimits: req.HardLimits,
		UpdatedBy:  actor,
		UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "tenant_settings.upsert",
		ResourceType: "tenant_settings",
		Payload: map[string]any{
			"plan_code": settings.PlanCode,
			"limits":    settings.HardLimits,
		},
	})
	return settings, nil
}

func (u *Usecases) ListActivity(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, limit int) ([]admindomain.AdminActivityEvent, error) {
	canRead, _ := adminCapabilities(role, scopes)
	if !canRead {
		return nil, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console read permission required")
	}
	items, err := u.repo.ListAdminActivityEvents(ctx, orgID, limit)
	if err != nil {
		return nil, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "admin.activity.read",
		ResourceType: "admin_activity_events",
		Payload: map[string]any{
			"limit": limit,
		},
	})
	return items, nil
}

func adminCapabilities(role *string, scopes []string) (bool, bool) {
	isPrivilegedRole := authz.IsRole(role, "admin", "secops")
	canRead := isPrivilegedRole || authz.HasAnyScope(scopes, scopeConsoleRead, scopeConsoleWrite)
	canWrite := authz.IsRole(role, "admin") || authz.HasAnyScope(scopes, scopeConsoleWrite)
	return canRead, canWrite
}

func (u *Usecases) getOrDefaultSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, error) {
	settings, ok, err := u.repo.GetTenantSettings(ctx, orgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	if ok {
		if settings.HardLimits == nil {
			settings.HardLimits = defaultHardLimits(settings.PlanCode)
		}
		return settings, nil
	}
	defaults := defaultHardLimits("starter")
	stored, err := u.repo.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   "starter",
		HardLimits: defaults,
	})
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	return stored, nil
}

func defaultHardLimits(planCode string) map[string]any {
	switch planCode {
	case "enterprise":
		return map[string]any{"tools_max": 250, "run_rpm": 5000, "audit_retention_days": 365}
	case "growth":
		return map[string]any{"tools_max": 75, "run_rpm": 1200, "audit_retention_days": 90}
	default:
		return map[string]any{"tools_max": 20, "run_rpm": 300, "audit_retention_days": 30}
	}
}
