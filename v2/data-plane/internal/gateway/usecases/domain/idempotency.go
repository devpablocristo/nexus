package domain

import "time"

type IdempotencyOutcome string

const (
	IdempotencyNew             IdempotencyOutcome = "NEW"
	IdempotencyReplay          IdempotencyOutcome = "REPLAY"
	IdempotencyInProgress      IdempotencyOutcome = "IN_PROGRESS"
	IdempotencyConflict        IdempotencyOutcome = "CONFLICT"
	IdempotencySkippedNotWrite IdempotencyOutcome = "SKIPPED_NOT_WRITE"
)

type IdempotencyMeta struct {
	Present bool
	Outcome IdempotencyOutcome
}

type IdempotencyRecordStatus string

const (
	IdempotencyStatusInProgress IdempotencyRecordStatus = "IN_PROGRESS"
	IdempotencyStatusCompleted  IdempotencyRecordStatus = "COMPLETED"
	IdempotencyStatusFailed     IdempotencyRecordStatus = "FAILED"
)

type IdempotencyResponseSnapshot struct {
	Decision   string
	Status     string
	Reason     string
	Result     any
	ErrorCode  string
	ErrorMsg   string
	HTTPStatus int
	IntentID   string
	ApprovalID string
}

type IdempotencyRecord struct {
	ToolName           string
	IdempotencyKey     string
	RequestFingerprint string
	Status             IdempotencyRecordStatus
	Response           IdempotencyResponseSnapshot
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
