package repository

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-gateway/internal/org/repository/models"
	"nexus-gateway/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindOrgIDByAPIKeyHash(ctx context.Context, apiKeyHash string) (uuid.UUID, string, error) {
	var row models.OrgAPIKey
	err := r.db.WithContext(ctx).
		Select("org_id", "api_key_hash").
		Where("api_key_hash = ?", apiKeyHash).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, "", types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid api key")
		}
		return uuid.Nil, "", err
	}
	if subtle.ConstantTimeCompare([]byte(row.APIKeyHash), []byte(apiKeyHash)) != 1 {
		return uuid.Nil, row.APIKeyHash, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid api key")
	}
	return row.OrgID, row.APIKeyHash, nil
}

// Helpers for seed/tests.
func (r *Repository) UpsertOrgByName(ctx context.Context, name string) (uuid.UUID, error) {
	var org models.Org
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&org).Error
	if err == nil {
		return org.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}
	org = models.Org{ID: uuid.New(), Name: name}
	if err := r.db.WithContext(ctx).Create(&org).Error; err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (r *Repository) UpsertAPIKey(ctx context.Context, orgID uuid.UUID, apiKeyHash, name string) error {
	// api_key_hash is unique.
	var existing models.OrgAPIKey
	err := r.db.WithContext(ctx).Where("api_key_hash = ?", apiKeyHash).Take(&existing).Error
	if err == nil {
		existing.OrgID = orgID
		existing.Name = name
		return r.db.WithContext(ctx).Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	row := models.OrgAPIKey{OrgID: orgID, APIKeyHash: apiKeyHash, Name: name}
	row.ID = uuid.New()
	return r.db.WithContext(ctx).Create(&row).Error
}
