package admin

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-core/internal/admin/repository/models"
	admindomain "nexus-core/internal/admin/usecases/domain"
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
	limits, _ := json.Marshal(s.HardLimits)
	row := models.TenantSettings{
		OrgID:      s.OrgID,
		PlanCode:   s.PlanCode,
		HardLimits: limits,
		UpdatedBy:  s.UpdatedBy,
	}
	if err := r.db.WithContext(ctx).Where("org_id = ?", s.OrgID).Assign(&row).FirstOrCreate(&row).Error; err != nil {
		return admindomain.TenantSettings{}, err
	}
	stored, _, err := r.GetTenantSettings(ctx, s.OrgID)
	if err != nil {
		return admindomain.TenantSettings{}, err
	}
	return stored, nil
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
	return admindomain.TenantSettings{
		OrgID:      m.OrgID,
		PlanCode:   m.PlanCode,
		HardLimits: limits,
		UpdatedBy:  m.UpdatedBy,
		UpdatedAt:  m.UpdatedAt,
		CreatedAt:  m.CreatedAt,
	}
}
