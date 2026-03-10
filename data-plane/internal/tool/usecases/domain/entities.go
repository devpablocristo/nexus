package domain

import (
	"time"

	"github.com/google/uuid"
)

type ToolKind string

const (
	ToolKindHTTP ToolKind = "http"
)

type ActionType string

const (
	ActionRead  ActionType = "read"
	ActionWrite ActionType = "write"
)

type Tool struct {
	ID               uuid.UUID
	OrgID            uuid.UUID
	Name             string
	Kind             ToolKind
	Description      *string
	Method           string
	URL              string
	InputSchemaJSON  []byte
	OutputSchemaJSON []byte
	ActionType       ActionType
	Classification   string
	Sensitivity      string
	RiskLevel        int
	Enabled          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
