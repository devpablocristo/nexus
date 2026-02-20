package domain

import (
	"time"

	"github.com/google/uuid"
)

type ScopeType string

type ActionType string

type Status string

const (
	ScopeTenant ScopeType = "tenant"
	ScopeTool   ScopeType = "tool"
	ScopeAgent  ScopeType = "agent"
	ScopeGlobal ScopeType = "global"

	ActionThrottleTenantRPM ActionType = "throttle_tenant_rpm"
	ActionThrottleToolRPM   ActionType = "throttle_tool_rpm"
	ActionQuarantineTool    ActionType = "quarantine_tool"
	ActionDisableTool       ActionType = "disable_tool"

	StatusActive     Status = "active"
	StatusExpired    Status = "expired"
	StatusRolledBack Status = "rolled_back"
)

type Action struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	ScopeType    ScopeType
	ScopeID      *string
	ActionType   ActionType
	Params       map[string]any
	TTLSeconds   int
	Status       Status
	EvidenceRefs []string
	CreatedAt    time.Time
	CreatedBy    *string
	RolledBackAt *time.Time
	RolledBackBy *string
}

type RuntimeOverrides struct {
	Deny               bool
	DenyReason         string
	TenantRPMOverride  *int
	ToolRPMOverride    *int
	ActiveActionIDs    []string
	AppliedActionTypes []string
}
