package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type IdempotencyRow struct {
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

func (IdempotencyRow) TableName() string { return "idempotency_keys" }
