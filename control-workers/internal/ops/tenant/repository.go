package tenant

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"control-workers/internal/ops/tenant/repository/models"
	tenantdomain "control-workers/internal/ops/tenant/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertProfile(ctx context.Context, in tenantdomain.TenantProfile) error {
	costRaw, _ := json.Marshal(in.CostModel)
	row := models.TenantProfile{
		OrgID:               in.OrgID,
		Tier:                string(in.Tier),
		MaxTTLSeconds:       in.MaxTTLSeconds,
		AutoMitigateEnabled: in.AutoMitigateEnabled,
		CostModelJSON:       datatypes.JSON(costRaw),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"tier":                  row.Tier,
			"max_ttl_seconds":       row.MaxTTLSeconds,
			"auto_mitigate_enabled": row.AutoMitigateEnabled,
			"cost_model_json":       row.CostModelJSON,
			"updated_at":            gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

func (r *Repository) GetProfile(ctx context.Context, orgID uuid.UUID) (tenantdomain.TenantProfile, error) {
	var row models.TenantProfile
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&row).Error; err != nil {
		return tenantdomain.TenantProfile{}, err
	}
	var cost map[string]any
	_ = json.Unmarshal(row.CostModelJSON, &cost)
	if cost == nil {
		cost = map[string]any{}
	}
	return tenantdomain.TenantProfile{
		OrgID:               row.OrgID,
		Tier:                tenantdomain.Tier(row.Tier),
		MaxTTLSeconds:       row.MaxTTLSeconds,
		AutoMitigateEnabled: row.AutoMitigateEnabled,
		CostModel:           cost,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}, nil
}

func (r *Repository) ListContacts(ctx context.Context, orgID uuid.UUID) ([]tenantdomain.Contact, error) {
	var rows []models.Contact
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("is_primary desc, created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]tenantdomain.Contact, 0, len(rows))
	for _, row := range rows {
		out = append(out, tenantdomain.Contact{
			ID:          row.ID,
			OrgID:       row.OrgID,
			Name:        row.Name,
			Channel:     row.Channel,
			Destination: row.Destination,
			SeverityMin: row.SeverityMin,
			IsPrimary:   row.IsPrimary,
			CreatedAt:   row.CreatedAt,
		})
	}
	return out, nil
}

func (r *Repository) UpsertIncidentSettings(ctx context.Context, in tenantdomain.IncidentSettings) error {
	thresholdRaw, _ := json.Marshal(in.AutoOpenThreshold)
	row := models.IncidentSettings{
		OrgID:                         in.OrgID,
		AutoOpenThresholdJSON:         datatypes.JSON(thresholdRaw),
		CooldownSeconds:               in.CooldownSeconds,
		MonitoringWindowSeconds:       in.MonitoringWindowSeconds,
		ExternalCommsRequiresApproval: in.ExternalCommsRequiresApproval,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "org_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"auto_open_threshold_json":          row.AutoOpenThresholdJSON,
			"cooldown_seconds":                  row.CooldownSeconds,
			"monitoring_window_seconds":         row.MonitoringWindowSeconds,
			"external_comms_requires_approval":  row.ExternalCommsRequiresApproval,
			"updated_at":                        gorm.Expr("now()"),
		}),
	}).Create(&row).Error
}

func (r *Repository) GetIncidentSettings(ctx context.Context, orgID uuid.UUID) (tenantdomain.IncidentSettings, error) {
	var row models.IncidentSettings
	err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantdomain.IncidentSettings{
				OrgID:                        orgID,
				AutoOpenThreshold:            map[string]any{},
				CooldownSeconds:              300,
				MonitoringWindowSeconds:      600,
				ExternalCommsRequiresApproval: true,
			}, nil
		}
		return tenantdomain.IncidentSettings{}, err
	}
	var threshold map[string]any
	_ = json.Unmarshal(row.AutoOpenThresholdJSON, &threshold)
	if threshold == nil {
		threshold = map[string]any{}
	}
	return tenantdomain.IncidentSettings{
		OrgID:                        row.OrgID,
		AutoOpenThreshold:            threshold,
		CooldownSeconds:              row.CooldownSeconds,
		MonitoringWindowSeconds:      row.MonitoringWindowSeconds,
		ExternalCommsRequiresApproval: row.ExternalCommsRequiresApproval,
		UpdatedAt:                    row.UpdatedAt,
	}, nil
}
