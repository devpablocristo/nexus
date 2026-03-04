package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	gwdomain "nexus-core/internal/gateway/usecases/domain"
	"nexus-core/internal/gateway/idempotency_repository/models"
)

type IdempotencyRepository struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

func (r *IdempotencyRepository) Get(ctx context.Context, orgID uuid.UUID, toolName, key string) (*gwdomain.IdempotencyRecord, error) {
	var row models.IdempotencyRow
	err := r.db.WithContext(ctx).
		Where("org_id = ? and tool_name = ? and idempotency_key = ?", orgID, toolName, key).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toIdempotencyDomain(row), nil
}

// CreateInProgress attempts an atomic insert with ON CONFLICT DO NOTHING.
// Returns (true, nil) if inserted, (false, nil) if a duplicate already exists.
func (r *IdempotencyRepository) CreateInProgress(ctx context.Context, rec gwdomain.IdempotencyRecord) (bool, error) {
	id := uuid.New()
	tx := r.db.WithContext(ctx).Exec(
		`INSERT INTO idempotency_keys (id, org_id, tool_name, idempotency_key, request_fingerprint, status, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (org_id, tool_name, idempotency_key) DO NOTHING`,
		id, rec.OrgID, rec.ToolName, rec.IdempotencyKey, rec.RequestFingerprint,
		string(gwdomain.IdempotencyStatusInProgress), rec.ExpiresAt,
	)
	if tx.Error != nil {
		return false, tx.Error
	}
	return tx.RowsAffected > 0, nil
}

func (r *IdempotencyRepository) MarkCompleted(ctx context.Context, orgID uuid.UUID, toolName, key string, responseRedacted map[string]any) error {
	body, err := json.Marshal(responseRedacted)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	return r.db.WithContext(ctx).
		Model(&models.IdempotencyRow{}).
		Where("org_id = ? and tool_name = ? and idempotency_key = ?", orgID, toolName, key).
		Updates(map[string]any{
			"status":                 string(gwdomain.IdempotencyStatusCompleted),
			"response_redacted_json": body,
			"error_code":             nil,
		}).Error
}

func (r *IdempotencyRepository) MarkFailed(ctx context.Context, orgID uuid.UUID, toolName, key string, code *string, responseRedacted map[string]any) error {
	body, err := json.Marshal(responseRedacted)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	return r.db.WithContext(ctx).
		Model(&models.IdempotencyRow{}).
		Where("org_id = ? and tool_name = ? and idempotency_key = ?", orgID, toolName, key).
		Updates(map[string]any{
			"status":                 string(gwdomain.IdempotencyStatusFailed),
			"error_code":             code,
			"response_redacted_json": body,
		}).Error
}

func (r *IdempotencyRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	tx := r.db.WithContext(ctx).Where("expires_at < ?", before).Delete(&models.IdempotencyRow{})
	return tx.RowsAffected, tx.Error
}

func toIdempotencyDomain(row models.IdempotencyRow) *gwdomain.IdempotencyRecord {
	out := &gwdomain.IdempotencyRecord{
		OrgID:              row.OrgID,
		ToolName:           row.ToolName,
		IdempotencyKey:     row.IdempotencyKey,
		RequestFingerprint: row.RequestFingerprint,
		Status:             gwdomain.IdempotencyRecordStatus(row.Status),
		ErrorCode:          row.ErrorCode,
		ExpiresAt:          row.ExpiresAt,
		CreatedAt:          row.CreatedAt,
	}
	if len(row.ResponseRedacted) > 0 {
		var m map[string]any
		_ = json.Unmarshal(row.ResponseRedacted, &m)
		out.ResponseRedacted = m
	}
	return out
}
