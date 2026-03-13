package secrets

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"data-plane/internal/secrets/repository/models"
	secretdomain "data-plane/internal/secrets/usecases/domain"
	"nexus/pkg/types"
	"nexus/pkg/utils"
)

type Repository struct {
	db     *gorm.DB
	crypto *utils.AESGCM
}

func NewRepository(db *gorm.DB, crypto *utils.AESGCM) *Repository {
	return &Repository{db: db, crypto: crypto}
}

func (r *Repository) UpsertForTool(ctx context.Context, orgID, toolID uuid.UUID, secret secretdomain.ToolSecret) (secretdomain.ToolSecret, error) {
	ciphertext, nonce, err := r.crypto.Encrypt([]byte(secret.PlaintextValue))
	if err != nil {
		return secretdomain.ToolSecret{}, types.NewHTTPError(500, types.ErrCodeCryptoFailed, "secret encryption failed")
	}
	var row models.ToolSecret
	err = r.db.WithContext(ctx).Where("org_id = ? AND tool_id = ? AND key_name = ?", orgID, toolID, secret.KeyName).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return secretdomain.ToolSecret{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.ToolSecret{ID: uuid.New(), OrgID: orgID, ToolID: toolID}
	}
	row.SecretType = secret.SecretType
	row.KeyName = secret.KeyName
	row.Ciphertext = ciphertext
	row.Nonce = nonce
	row.Enabled = secret.Enabled
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return secretdomain.ToolSecret{}, err
		}
	} else {
		if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
			return secretdomain.ToolSecret{}, err
		}
	}
	return secretdomain.ToolSecret{ID: row.ID, OrgID: row.OrgID, ToolID: row.ToolID, SecretType: row.SecretType, KeyName: row.KeyName, Enabled: row.Enabled, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) ListForTool(ctx context.Context, orgID, toolID uuid.UUID) ([]secretdomain.ToolSecret, error) {
	var rows []models.ToolSecret
	if err := r.db.WithContext(ctx).Where("org_id = ? AND tool_id = ?", orgID, toolID).Order("key_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]secretdomain.ToolSecret, 0, len(rows))
	for _, row := range rows {
		pt, err := r.crypto.Decrypt(row.Ciphertext, row.Nonce)
		if err != nil {
			return nil, types.NewHTTPError(500, types.ErrCodeCryptoFailed, "secret decryption failed")
		}
		out = append(out, secretdomain.ToolSecret{
			ID:             row.ID,
			OrgID:          row.OrgID,
			ToolID:         row.ToolID,
			SecretType:     row.SecretType,
			KeyName:        row.KeyName,
			PlaintextValue: string(pt),
			Enabled:        row.Enabled,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	return out, nil
}

func (r *Repository) DeleteForTool(ctx context.Context, orgID, toolID uuid.UUID, keyName string) error {
	tx := r.db.WithContext(ctx).Where("org_id = ? AND tool_id = ?", orgID, toolID)
	if keyName != "" {
		tx = tx.Where("key_name = ?", keyName)
	}
	return tx.Delete(&models.ToolSecret{}).Error
}
