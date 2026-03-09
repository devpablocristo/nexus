package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nexus-saas/internal/admin/repository/models"
	admindomain "nexus-saas/internal/admin/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTenantSettings(ctx context.Context, orgID uuid.UUID) (admindomain.TenantSettings, bool, error) {
	var row models.TenantSettings
	err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return admindomain.TenantSettings{}, false, nil
		}
		return admindomain.TenantSettings{}, false, err
	}
	return toTenantDomain(row), true, nil
}

func (r *Repository) UpsertTenantSettings(ctx context.Context, s admindomain.TenantSettings) (admindomain.TenantSettings, error) {
	status := strings.TrimSpace(strings.ToLower(s.Status))
	if status == "" {
		status = admindomain.TenantStatusActive
	}
	limits, _ := json.Marshal(s.HardLimits)
	row := models.TenantSettings{
		OrgID:      s.OrgID,
		PlanCode:   s.PlanCode,
		Status:     status,
		DeletedAt:  s.DeletedAt,
		HardLimits: limits,
		UpdatedBy:  s.UpdatedBy,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "org_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"plan_code":        row.PlanCode,
				"hard_limits_json": row.HardLimits,
				"updated_by":       row.UpdatedBy,
				"updated_at":       gorm.Expr("NOW()"),
			}),
		}).
		Create(&row).Error; err != nil {
		return admindomain.TenantSettings{}, err
	}
	stored, _, err := r.GetTenantSettings(ctx, s.OrgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	return stored, nil
}

func (r *Repository) UpdateTenantLifecycle(ctx context.Context, orgID uuid.UUID, status string, deletedAt *time.Time, updatedBy *string) (admindomain.TenantSettings, error) {
	updates := map[string]any{
		"status":     status,
		"deleted_at": deletedAt,
		"updated_by": updatedBy,
		"updated_at": gorm.Expr("NOW()"),
	}
	res := r.db.WithContext(ctx).
		Model(&models.TenantSettings{}).
		Where("org_id = ?", orgID).
		Updates(updates)
	if res.Error != nil {
		return admindomain.TenantSettings{}, res.Error
	}
	if res.RowsAffected == 0 {
		return admindomain.TenantSettings{}, gorm.ErrRecordNotFound
	}
	settings, _, err := r.GetTenantSettings(ctx, orgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	return settings, nil
}

func (r *Repository) CreateAdminActivityEvent(ctx context.Context, ev admindomain.AdminActivityEvent) error {
	payload, _ := json.Marshal(ev.Payload)
	row := models.AdminActivityEvent{
		ID:           uuid.New(),
		OrgID:        ev.OrgID,
		Actor:        ev.Actor,
		Action:       ev.Action,
		ResourceType: ev.ResourceType,
		ResourceID:   ev.ResourceID,
		Payload:      payload,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) ListAdminActivityEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]admindomain.AdminActivityEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	var rows []models.AdminActivityEvent
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]admindomain.AdminActivityEvent, 0, len(rows))
	for _, row := range rows {
		var payload map[string]any
		_ = json.Unmarshal(row.Payload, &payload)
		if payload == nil {
			payload = map[string]any{}
		}
		out = append(out, admindomain.AdminActivityEvent{
			ID:           row.ID,
			OrgID:        row.OrgID,
			Actor:        row.Actor,
			Action:       row.Action,
			ResourceType: row.ResourceType,
			ResourceID:   row.ResourceID,
			Payload:      payload,
			CreatedAt:    row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) ListProtectedResources(ctx context.Context, orgID uuid.UUID) ([]admindomain.ProtectedResource, error) {
	var rows []models.ProtectedResource
	if err := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]admindomain.ProtectedResource, 0, len(rows))
	for _, row := range rows {
		out = append(out, admindomain.ProtectedResource{
			ID:           row.ID,
			OrgID:        row.OrgID,
			Name:         row.Name,
			ResourceType: row.ResourceType,
			MatchValue:   row.MatchValue,
			MatchMode:    row.MatchMode,
			Environment:  row.Environment,
			Reason:       row.Reason,
			Enabled:      row.Enabled,
			CreatedBy:    row.CreatedBy,
			UpdatedBy:    row.UpdatedBy,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CreateProtectedResource(ctx context.Context, resource admindomain.ProtectedResource) (admindomain.ProtectedResource, error) {
	row := models.ProtectedResource{
		ID:           resource.ID,
		OrgID:        resource.OrgID,
		Name:         resource.Name,
		ResourceType: resource.ResourceType,
		MatchValue:   resource.MatchValue,
		MatchMode:    resource.MatchMode,
		Environment:  resource.Environment,
		Reason:       resource.Reason,
		Enabled:      resource.Enabled,
		CreatedBy:    resource.CreatedBy,
		UpdatedBy:    resource.UpdatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return admindomain.ProtectedResource{}, err
	}
	return admindomain.ProtectedResource{
		ID:           row.ID,
		OrgID:        row.OrgID,
		Name:         row.Name,
		ResourceType: row.ResourceType,
		MatchValue:   row.MatchValue,
		MatchMode:    row.MatchMode,
		Environment:  row.Environment,
		Reason:       row.Reason,
		Enabled:      row.Enabled,
		CreatedBy:    row.CreatedBy,
		UpdatedBy:    row.UpdatedBy,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

func (r *Repository) DeleteProtectedResource(ctx context.Context, orgID, resourceID uuid.UUID) error {
	res := r.db.WithContext(ctx).
		Where("org_id = ? AND id = ?", orgID, resourceID).
		Delete(&models.ProtectedResource{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ListRestoreEvidence(ctx context.Context, orgID uuid.UUID, environment string, limit int) ([]admindomain.RestoreEvidence, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	query := r.db.WithContext(ctx).
		Where("org_id = ?", orgID)
	if env := strings.TrimSpace(strings.ToLower(environment)); env != "" {
		query = query.Where("environment = ?", env)
	}
	var rows []models.RestoreEvidence
	if err := query.Order("created_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]admindomain.RestoreEvidence, 0, len(rows))
	for _, row := range rows {
		var summary map[string]any
		_ = json.Unmarshal(row.Summary, &summary)
		if summary == nil {
			summary = map[string]any{}
		}
		out = append(out, admindomain.RestoreEvidence{
			ID:             row.ID,
			OrgID:          row.OrgID,
			Environment:    row.Environment,
			System:         row.System,
			Status:         row.Status,
			SnapshotID:     row.SnapshotID,
			RestoreTarget:  row.RestoreTarget,
			StartedAt:      row.StartedAt,
			CompletedAt:    row.CompletedAt,
			Source:         row.Source,
			ArtifactSHA256: row.ArtifactSHA256,
			Summary:        summary,
			CreatedAt:      row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) CreateRestoreEvidence(ctx context.Context, evidence admindomain.RestoreEvidence) (admindomain.RestoreEvidence, error) {
	summaryJSON, _ := json.Marshal(evidence.Summary)
	row := models.RestoreEvidence{
		ID:             evidence.ID,
		OrgID:          evidence.OrgID,
		Environment:    evidence.Environment,
		System:         evidence.System,
		Status:         evidence.Status,
		SnapshotID:     evidence.SnapshotID,
		RestoreTarget:  evidence.RestoreTarget,
		StartedAt:      evidence.StartedAt,
		CompletedAt:    evidence.CompletedAt,
		Source:         evidence.Source,
		ArtifactSHA256: evidence.ArtifactSHA256,
		Summary:        summaryJSON,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return admindomain.RestoreEvidence{}, err
	}
	return admindomain.RestoreEvidence{
		ID:             row.ID,
		OrgID:          row.OrgID,
		Environment:    row.Environment,
		System:         row.System,
		Status:         row.Status,
		SnapshotID:     row.SnapshotID,
		RestoreTarget:  row.RestoreTarget,
		StartedAt:      row.StartedAt,
		CompletedAt:    row.CompletedAt,
		Source:         row.Source,
		ArtifactSHA256: row.ArtifactSHA256,
		Summary:        evidence.Summary,
		CreatedAt:      row.CreatedAt,
	}, nil
}

func (r *Repository) GetRunRPM(ctx context.Context, orgID uuid.UUID) (int, error) {
	settings, ok, err := r.GetTenantSettings(ctx, orgID)
	if err != nil || !ok || settings.HardLimits == nil {
		return 0, err
	}
	v, ok := settings.HardLimits["run_rpm"]
	if !ok {
		return 0, nil
	}
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	default:
		// ignore invalid type and keep runtime stable
		return 0, nil
	}
}

func (r *Repository) GetToolsMax(ctx context.Context, orgID uuid.UUID) (int, error) {
	settings, ok, err := r.GetTenantSettings(ctx, orgID)
	if err != nil || !ok || settings.HardLimits == nil {
		return 0, err
	}
	v, ok := settings.HardLimits["tools_max"]
	if !ok {
		return 0, nil
	}
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case int64:
		return int(t), nil
	default:
		return 0, nil
	}
}

func toTenantDomain(m models.TenantSettings) admindomain.TenantSettings {
	var limits map[string]any
	_ = json.Unmarshal(m.HardLimits, &limits)
	if limits == nil {
		limits = map[string]any{}
	}
	status := strings.TrimSpace(strings.ToLower(m.Status))
	if status == "" {
		status = admindomain.TenantStatusActive
	}
	return admindomain.TenantSettings{
		OrgID:      m.OrgID,
		PlanCode:   m.PlanCode,
		Status:     status,
		DeletedAt:  m.DeletedAt,
		HardLimits: limits,
		UpdatedBy:  m.UpdatedBy,
		UpdatedAt:  m.UpdatedAt,
		CreatedAt:  m.CreatedAt,
	}
}
