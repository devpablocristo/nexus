package admin

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	admindomain "control-plane/internal/admin/usecases/domain"
	"control-plane/internal/shared/authz"
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
	ListProtectedResources(ctx context.Context, orgID uuid.UUID) ([]admindomain.ProtectedResource, error)
	CreateProtectedResource(ctx context.Context, resource admindomain.ProtectedResource) (admindomain.ProtectedResource, error)
	DeleteProtectedResource(ctx context.Context, orgID, resourceID uuid.UUID) error
	ListRestoreEvidence(ctx context.Context, orgID uuid.UUID, environment string, limit int) ([]admindomain.RestoreEvidence, error)
	CreateRestoreEvidence(ctx context.Context, evidence admindomain.RestoreEvidence) (admindomain.RestoreEvidence, error)
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

type CreateProtectedResourceRequest struct {
	Name         string
	ResourceType string
	MatchValue   string
	MatchMode    string
	Environment  string
	Reason       string
	Enabled      *bool
}

type RecordRestoreEvidenceRequest struct {
	Environment    string
	System         string
	Status         string
	SnapshotID     string
	RestoreTarget  string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Source         string
	ArtifactSHA256 string
	Summary        map[string]any
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

func (u *Usecases) ListProtectedResources(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string) ([]admindomain.ProtectedResource, error) {
	canRead, _ := adminCapabilities(role, scopes)
	if !canRead {
		return nil, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console read permission required")
	}
	items, err := u.repo.ListProtectedResources(ctx, orgID)
	if err != nil {
		return nil, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "protected_resources.read",
		ResourceType: "protected_resource",
		Payload: map[string]any{
			"count": len(items),
		},
	})
	return items, nil
}

func (u *Usecases) CreateProtectedResource(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, req CreateProtectedResourceRequest) (admindomain.ProtectedResource, error) {
	_, canWrite := adminCapabilities(role, scopes)
	if !canWrite {
		return admindomain.ProtectedResource{}, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console write permission required")
	}
	name := strings.TrimSpace(req.Name)
	resourceType := strings.TrimSpace(strings.ToLower(req.ResourceType))
	matchValue := strings.TrimSpace(req.MatchValue)
	matchMode := strings.TrimSpace(strings.ToLower(req.MatchMode))
	if matchMode == "" {
		matchMode = admindomain.ProtectedResourceMatchExact
	}
	environment := strings.TrimSpace(strings.ToLower(req.Environment))
	if environment == "" {
		environment = "*"
	}
	reason := strings.TrimSpace(req.Reason)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if name == "" || resourceType == "" || matchValue == "" {
		return admindomain.ProtectedResource{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "name, resource_type, and match_value are required")
	}
	switch matchMode {
	case admindomain.ProtectedResourceMatchExact, admindomain.ProtectedResourceMatchContains:
	default:
		return admindomain.ProtectedResource{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "match_mode must be exact or contains")
	}
	switch environment {
	case "*", "prod", "nonprod":
	default:
		return admindomain.ProtectedResource{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "environment must be *, prod, or nonprod")
	}
	resource, err := u.repo.CreateProtectedResource(ctx, admindomain.ProtectedResource{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         name,
		ResourceType: resourceType,
		MatchValue:   matchValue,
		MatchMode:    matchMode,
		Environment:  environment,
		Reason:       reason,
		Enabled:      enabled,
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		return admindomain.ProtectedResource{}, err
	}
	resourceID := resource.ID.String()
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "protected_resource.create",
		ResourceType: "protected_resource",
		ResourceID:   &resourceID,
		Payload: map[string]any{
			"name":          resource.Name,
			"resource_type": resource.ResourceType,
			"match_mode":    resource.MatchMode,
			"environment":   resource.Environment,
			"enabled":       resource.Enabled,
		},
	})
	return resource, nil
}

func (u *Usecases) DeleteProtectedResource(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, resourceID uuid.UUID) error {
	_, canWrite := adminCapabilities(role, scopes)
	if !canWrite {
		return types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console write permission required")
	}
	if resourceID == uuid.Nil {
		return types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "invalid protected resource id")
	}
	if err := u.repo.DeleteProtectedResource(ctx, orgID, resourceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "protected resource not found")
		}
		return err
	}
	id := resourceID.String()
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "protected_resource.delete",
		ResourceType: "protected_resource",
		ResourceID:   &id,
	})
	return nil
}

func (u *Usecases) ListRestoreEvidence(ctx context.Context, orgID uuid.UUID, actor, role *string, scopes []string, environment string, limit int) ([]admindomain.RestoreEvidence, error) {
	canRead, _ := adminCapabilities(role, scopes)
	if !canRead {
		return nil, types.NewHTTPError(http.StatusForbidden, types.ErrCodeUnauthorized, "admin console read permission required")
	}
	items, err := u.repo.ListRestoreEvidence(ctx, orgID, environment, limit)
	if err != nil {
		return nil, err
	}
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "restore_evidence.read",
		ResourceType: "restore_evidence",
		Payload: map[string]any{
			"count":       len(items),
			"environment": strings.TrimSpace(strings.ToLower(environment)),
			"limit":       limit,
		},
	})
	return items, nil
}

func (u *Usecases) RecordRestoreEvidence(ctx context.Context, orgID uuid.UUID, actor *string, req RecordRestoreEvidenceRequest) (admindomain.RestoreEvidence, error) {
	environment := strings.TrimSpace(strings.ToLower(req.Environment))
	if environment == "" {
		environment = "prod"
	}
	system := strings.TrimSpace(strings.ToLower(req.System))
	if system == "" {
		system = "database"
	}
	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status == "" {
		status = admindomain.RestoreEvidenceStatusPassed
	}
	switch environment {
	case "prod", "nonprod", "*":
	default:
		return admindomain.RestoreEvidence{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "environment must be prod, nonprod, or *")
	}
	switch status {
	case admindomain.RestoreEvidenceStatusPassed, admindomain.RestoreEvidenceStatusFailed:
	default:
		return admindomain.RestoreEvidence{}, types.NewHTTPError(http.StatusBadRequest, types.ErrCodeValidation, "status must be passed or failed")
	}
	evidence, err := u.repo.CreateRestoreEvidence(ctx, admindomain.RestoreEvidence{
		ID:             uuid.New(),
		OrgID:          orgID,
		Environment:    environment,
		System:         system,
		Status:         status,
		SnapshotID:     strings.TrimSpace(req.SnapshotID),
		RestoreTarget:  strings.TrimSpace(req.RestoreTarget),
		StartedAt:      req.StartedAt,
		CompletedAt:    req.CompletedAt,
		Source:         strings.TrimSpace(req.Source),
		ArtifactSHA256: strings.TrimSpace(req.ArtifactSHA256),
		Summary:        cloneStringAnyMap(req.Summary),
	})
	if err != nil {
		return admindomain.RestoreEvidence{}, err
	}
	resourceID := evidence.ID.String()
	_ = u.repo.CreateAdminActivityEvent(ctx, admindomain.AdminActivityEvent{
		OrgID:        orgID,
		Actor:        actor,
		Action:       "restore_evidence.record",
		ResourceType: "restore_evidence",
		ResourceID:   &resourceID,
		Payload: map[string]any{
			"environment":     evidence.Environment,
			"system":          evidence.System,
			"status":          evidence.Status,
			"snapshot_id":     evidence.SnapshotID,
			"restore_target":  evidence.RestoreTarget,
			"artifact_sha256": evidence.ArtifactSHA256,
			"source":          evidence.Source,
		},
	})
	return evidence, nil
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

func cloneStringAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
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
