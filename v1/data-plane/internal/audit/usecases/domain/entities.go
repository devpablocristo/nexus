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
	ID                         uuid.UUID
	OrgID                      uuid.UUID
	ToolID                     uuid.UUID
	ToolName                   string
	RequestID                  string
	Actor                      *string
	ActorRole                  *string
	ActorScopes                []string
	InputRedacted              any
	ContextRedacted            any
	DLPSummary                 any
	Decision                   Decision
	PolicyID                   *uuid.UUID
	Reason                     *string
	Status                     Status
	OutputRedacted             any
	ErrorCode                  *string
	ErrorMessage               *string
	LatencyMS                  int
	IdempotencyPresent         bool
	IdempotencyOutcome         string
	TimeoutMS                  *int
	BudgetRemainingMSAtExecute *int
	StageDurationsMS           map[string]int64
	PrevEventHash              *string
	EventHash                  *string
	CreatedAt                  time.Time
}

type Query struct {
	ToolName *string
	Decision *Decision
	Status   *Status
	From     *time.Time
	To       *time.Time
	Limit    int
	OrderAsc bool
}
