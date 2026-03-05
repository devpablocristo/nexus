package admin

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	admindomain "nexus-saas/internal/admin/usecases/domain"
	"nexus-saas/internal/shared/authz"
	"nexus/pkg/types"
)

const (
	scopeConsoleRead  = "admin:console:read"
	scopeConsoleWrite = "admin:console:write"
)

type RepositoryPort interface {
	GetTenantSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, bool, error)
	UpsertTenantSettings(ctx context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error)
	UpdateTenantLifecycle(ctx context.Context, orgID uuid.UUID, status string, deletedAt *time.Time, updatedBy *string) (admindomain.TenantSettings, error)
	CreateAdminActivityEvent(ctx context.Context, ev admindomain.AdminActivityEvent) error
	ListAdminActivityEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]admindomain.AdminActivityEvent, error)
}

type NotificationPort interface {
	Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
}

type Usecases struct {
	repo          RepositoryPort
	notifications NotificationPort
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

func NewUsecasesWithNotifications(repo RepositoryPort, notifications NotificationPort) *Usecases {
	return &Usecases{
		repo:          repo,
		notifications: notifications,
	}
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
	status := admindomain.TenantStatusActive
	var deletedAt *time.Time
	if current, ok, err := u.repo.GetTenantSettings(ctx, orgID); err == nil && ok {
		if strings.TrimSpace(current.Status) != "" {
			status = current.Status
		}
		deletedAt = current.DeletedAt
	}
	settings, err := u.repo.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   req.PlanCode,
		Status:     status,
		DeletedAt:  deletedAt,
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

func (u *Usecases) SuspendTenant(ctx context.Context, actorOrgID, targetOrgID uuid.UUID, actor, role *string, scopes []string) (admindomain.TenantSettings, error) {
	if err := ensureWritableTargetOrg(actorOrgID, targetOrgID, role, scopes); err != nil {
		return admindomain.TenantSettings{}, err
	}
	settings, err := u.getOrDefaultSettings(ctx, targetOrgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	stored, err := u.repo.UpdateTenantLifecycle(ctx, targetOrgID, admindomain.TenantStatusSuspended, nil, actor)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        targetOrgID,
		Actor:        actor,
		Action:       "tenant.suspend",
		ResourceType: "tenant_settings",
		Payload: map[string]any{
			"previous_status": settings.Status,
			"status":          stored.Status,
		},
	})
	u.notifyTenantAsync(targetOrgID, "tenant_suspended", map[string]string{
		"org_id":       targetOrgID.String(),
		"plan_code":    stored.PlanCode,
		"action_url":   "/billing",
		"reference_id": targetOrgID.String() + ":suspend",
	})
	return stored, nil
}

func (u *Usecases) ReactivateTenant(ctx context.Context, actorOrgID, targetOrgID uuid.UUID, actor, role *string, scopes []string) (admindomain.TenantSettings, error) {
	if err := ensureWritableTargetOrg(actorOrgID, targetOrgID, role, scopes); err != nil {
		return admindomain.TenantSettings{}, err
	}
	settings, err := u.getOrDefaultSettings(ctx, targetOrgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	stored, err := u.repo.UpdateTenantLifecycle(ctx, targetOrgID, admindomain.TenantStatusActive, nil, actor)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        targetOrgID,
		Actor:        actor,
		Action:       "tenant.reactivate",
		ResourceType: "tenant_settings",
		Payload: map[string]any{
			"previous_status": settings.Status,
			"status":          stored.Status,
		},
	})
	u.notifyTenantAsync(targetOrgID, "tenant_reactivated", map[string]string{
		"org_id":       targetOrgID.String(),
		"plan_code":    stored.PlanCode,
		"action_url":   "/tools",
		"reference_id": targetOrgID.String() + ":reactivate",
	})
	return stored, nil
}

func (u *Usecases) DeleteTenant(ctx context.Context, actorOrgID, targetOrgID uuid.UUID, actor, role *string, scopes []string) (admindomain.TenantSettings, error) {
	if err := ensureWritableTargetOrg(actorOrgID, targetOrgID, role, scopes); err != nil {
		return admindomain.TenantSettings{}, err
	}
	settings, err := u.getOrDefaultSettings(ctx, targetOrgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	now := time.Now().UTC()
	stored, err := u.repo.UpdateTenantLifecycle(ctx, targetOrgID, admindomain.TenantStatusDeleted, &now, actor)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        targetOrgID,
		Actor:        actor,
		Action:       "tenant.soft_delete",
		ResourceType: "tenant_settings",
		Payload: map[string]any{
			"previous_status": settings.Status,
			"status":          stored.Status,
			"deleted_at":      now.Format(time.RFC3339),
			"retention":       "30d",
			"note":            "hard-delete job should purge data after retention window",
		},
	})
	return stored, nil
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

func ensureWritableTargetOrg(actorOrgID, targetOrgID uuid.UUID, role *string, scopes []string) error {
	_, canWrite := adminCapabilities(role, scopes)
	if !canWrite {
		return types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console write permission required")
	}
	if actorOrgID == uuid.Nil || targetOrgID == uuid.Nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid org_id")
	}
	if actorOrgID != targetOrgID {
		return types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "cross-tenant admin lifecycle is not allowed")
	}
	return nil
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
		if strings.TrimSpace(settings.Status) == "" {
			settings.Status = admindomain.TenantStatusActive
		}
		return settings, nil
	}
	defaults := defaultHardLimits("starter")
	stored, err := u.repo.UpsertTenantSettings(ctx, admindomain.TenantSettings{
		OrgID:      orgID,
		PlanCode:   "starter",
		Status:     admindomain.TenantStatusActive,
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

// DefaultHardLimits expone los límites base por plan para módulos internos
// que necesitan sincronizar cambios de plan sin duplicar reglas.
func DefaultHardLimits(planCode string) map[string]any {
	return defaultHardLimits(planCode)
}

func (u *Usecases) notifyTenantAsync(orgID uuid.UUID, notifType string, data map[string]string) {
	if u.notifications == nil {
		return
	}
	payload := map[string]string{}
	for k, v := range data {
		payload[k] = v
	}
	go func() {
		_ = u.notifications.Notify(context.Background(), orgID, notifType, payload)
	}()
}

// AutoSuspend suspends a tenant without interactive actor context
// (e.g. dunning worker after grace period).
func (u *Usecases) AutoSuspend(ctx context.Context, targetOrgID uuid.UUID) error {
	if targetOrgID == uuid.Nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid org_id")
	}
	settings, err := u.getOrDefaultSettings(ctx, targetOrgID)
	if err != nil {
		return err
	}
	if settings.Status == admindomain.TenantStatusSuspended || settings.Status == admindomain.TenantStatusDeleted {
		return nil
	}
	systemActor := "system:dunning-worker"
	stored, err := u.repo.UpdateTenantLifecycle(ctx, targetOrgID, admindomain.TenantStatusSuspended, nil, &systemActor)
	if err != nil {
		return err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        targetOrgID,
		Actor:        &systemActor,
		Action:       "tenant.auto_suspend",
		ResourceType: "tenant_settings",
		Payload: map[string]any{
			"previous_status": settings.Status,
			"status":          stored.Status,
			"reason":          "billing_grace_period_expired",
		},
	})
	u.notifyTenantAsync(targetOrgID, "tenant_suspended", map[string]string{
		"org_id":       targetOrgID.String(),
		"plan_code":    stored.PlanCode,
		"action_url":   "/billing",
		"reference_id": targetOrgID.String() + ":auto-suspend",
	})
	return nil
}
