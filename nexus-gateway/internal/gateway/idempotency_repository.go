package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	gwdomain "nexus-gateway/internal/gateway/usecases/domain"
)

type idempotencyRow struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrgID              uuid.UUID      `gorm:"type:uuid;index"`
	ToolName           string         `gorm:"index"`
	IdempotencyKey     string         `gorm:"column:idempotency_key"`
	RequestFingerprint string         `gorm:"column:request_fingerprint"`
	Status             string         `gorm:"column:status"`
	ResponseRedacted   datatypes.JSON `gorm:"column:response_redacted_json;type:jsonb"`
	ErrorCode          *string        `gorm:"column:error_code"`
	CreatedAt          time.Time      `gorm:"column:created_at"`
	ExpiresAt          time.Time      `gorm:"column:expires_at"`
}

func (idempotencyRow) TableName() string { return "idempotency_keys" }

type IdempotencyRepository struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

func (r *IdempotencyRepository) Get(ctx context.Context, orgID uuid.UUID, toolName, key string) (*gwdomain.IdempotencyRecord, error) {
	var row idempotencyRow
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

func (r *IdempotencyRepository) CreateInProgress(ctx context.Context, rec gwdomain.IdempotencyRecord) error {
	row := idempotencyRow{
		ID:                 uuid.New(),
		OrgID:              rec.OrgID,
		ToolName:           rec.ToolName,
		IdempotencyKey:     rec.IdempotencyKey,
		RequestFingerprint: rec.RequestFingerprint,
		Status:             string(gwdomain.IdempotencyStatusInProgress),
		ExpiresAt:          rec.ExpiresAt,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *IdempotencyRepository) MarkCompleted(ctx context.Context, orgID uuid.UUID, toolName, key string, responseRedacted map[string]any) error {
	body, err := json.Marshal(responseRedacted)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	return r.db.WithContext(ctx).
		Model(&idempotencyRow{}).
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
		Model(&idempotencyRow{}).
		Where("org_id = ? and tool_name = ? and idempotency_key = ?", orgID, toolName, key).
		Updates(map[string]any{
			"status":                 string(gwdomain.IdempotencyStatusFailed),
			"error_code":             code,
			"response_redacted_json": body,
		}).Error
}

func (r *IdempotencyRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	tx := r.db.WithContext(ctx).Where("expires_at < ?", before).Delete(&idempotencyRow{})
	return tx.RowsAffected, tx.Error
}

func toIdempotencyDomain(row idempotencyRow) *gwdomain.IdempotencyRecord {
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
