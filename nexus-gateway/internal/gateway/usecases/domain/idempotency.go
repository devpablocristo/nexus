package domain

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecordStatus string

const (
	IdempotencyStatusInProgress IdempotencyRecordStatus = "IN_PROGRESS"
	IdempotencyStatusCompleted  IdempotencyRecordStatus = "COMPLETED"
	IdempotencyStatusFailed     IdempotencyRecordStatus = "FAILED"
)

type IdempotencyRecord struct {
	OrgID              uuid.UUID
	ToolName           string
	IdempotencyKey     string
	RequestFingerprint string
	Status             IdempotencyRecordStatus
	ResponseRedacted   map[string]any
	ErrorCode          *string
	ExpiresAt          time.Time
}
