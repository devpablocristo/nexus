package domain

import (
	"time"

	"github.com/google/uuid"
)

type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
	StatusBlocked Status = "blocked"
)

type AuditEvent struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	ToolID          uuid.UUID
	ToolName        string
	RequestID       string
	Actor           *string
	InputRedacted   any
	ContextRedacted any
	Decision        Decision
	PolicyID        *uuid.UUID
	Reason          *string
	Status          Status
	OutputRedacted  any
	ErrorCode       *string
	ErrorMessage    *string
	LatencyMS       int
	CreatedAt       time.Time
}

type Query struct {
	ToolName *string
	Decision *Decision
	Status   *Status
	From     *time.Time
	To       *time.Time
	Limit    int
}
