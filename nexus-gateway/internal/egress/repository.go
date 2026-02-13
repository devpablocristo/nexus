package egress

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-gateway/internal/egress/repository/models"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Upsert(ctx context.Context, orgID, toolID uuid.UUID, host string, enabled bool) error {
	var row models.Rule
	err := r.db.WithContext(ctx).Where("org_id = ? AND tool_id = ? AND host = ?", orgID, toolID, host).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.Rule{ID: uuid.New(), OrgID: orgID, ToolID: toolID, Host: host, Enabled: enabled}
		return r.db.WithContext(ctx).Create(&row).Error
	}
	row.Enabled = enabled
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *Repository) List(ctx context.Context, orgID, toolID uuid.UUID) ([]string, error) {
	var hosts []string
	err := r.db.WithContext(ctx).Model(&models.Rule{}).
		Where("org_id = ? AND tool_id = ? AND enabled = true", orgID, toolID).
		Order("host asc").Pluck("host", &hosts).Error
	return hosts, err
}

func (r *Repository) Delete(ctx context.Context, orgID, toolID uuid.UUID, host string) error {
	return r.db.WithContext(ctx).Where("org_id = ? AND tool_id = ? AND host = ?", orgID, toolID, host).Delete(&models.Rule{}).Error
}

func (r *Repository) HasAny(ctx context.Context, orgID, toolID uuid.UUID) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Rule{}).Where("org_id = ? AND tool_id = ? AND enabled = true", orgID, toolID).Count(&n).Error
	return n > 0, err
}

func (r *Repository) ExistsHost(ctx context.Context, orgID, toolID uuid.UUID, host string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Rule{}).Where("org_id = ? AND tool_id = ? AND host = ? AND enabled = true", orgID, toolID, host).Count(&n).Error
	return n > 0, err
}
