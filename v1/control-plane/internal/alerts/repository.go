package alerts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"control-plane/internal/alerts/repository/models"
	domain "control-plane/internal/alerts/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	row := toModel(rule)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.AlertRule{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.AlertRule, error) {
	var rows []models.AlertRule
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.AlertRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) ListEnabled(ctx context.Context) ([]domain.AlertRule, error) {
	var rows []models.AlertRule
	if err := r.db.WithContext(ctx).Where("enabled = true").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.AlertRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ? AND org_id = ?", id, orgID).Delete(&models.AlertRule{}).Error
}

func (r *Repository) MarkFired(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.AlertRule{}).Where("id = ?", id).Update("last_fired_at", now).Error
}

func toModel(d domain.AlertRule) models.AlertRule {
	return models.AlertRule{
		ID:              d.ID,
		OrgID:           d.OrgID,
		Name:            d.Name,
		Metric:          string(d.Metric),
		Threshold:       d.Threshold,
		WindowSeconds:   d.WindowSeconds,
		ToolName:        d.ToolName,
		WebhookURL:      d.WebhookURL,
		CooldownSeconds: d.CooldownSeconds,
		Enabled:         d.Enabled,
		LastFiredAt:     d.LastFiredAt,
	}
}

func toDomain(m models.AlertRule) domain.AlertRule {
	return domain.AlertRule{
		ID:              m.ID,
		OrgID:           m.OrgID,
		Name:            m.Name,
		Metric:          domain.Metric(m.Metric),
		Threshold:       m.Threshold,
		WindowSeconds:   m.WindowSeconds,
		ToolName:        m.ToolName,
		WebhookURL:      m.WebhookURL,
		CooldownSeconds: m.CooldownSeconds,
		Enabled:         m.Enabled,
		LastFiredAt:     m.LastFiredAt,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}
