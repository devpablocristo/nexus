package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Event struct {
	Sequence        int64          `gorm:"column:sequence;primaryKey;autoIncrement"`
	ID              uuid.UUID      `gorm:"column:id;type:uuid"`
	EventType       string         `gorm:"column:event_type"`
	Version         int            `gorm:"column:version"`
	OccurredAt      time.Time      `gorm:"column:occurred_at"`
	OrgID           uuid.UUID      `gorm:"column:org_id;type:uuid"`
	CorrelationJSON datatypes.JSON `gorm:"column:correlation_json"`
	ActorJSON       datatypes.JSON `gorm:"column:actor_json"`
	Source          string         `gorm:"column:source"`
	PayloadJSON     datatypes.JSON `gorm:"column:payload_json"`
	SchemaValid     bool           `gorm:"column:schema_valid"`
	ValidationError *string        `gorm:"column:validation_error"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
}

func (Event) TableName() string { return "ops_event_store" }

type Contract struct {
	ID         uuid.UUID      `gorm:"column:id;type:uuid;primaryKey"`
	EventType  string         `gorm:"column:event_type"`
	Version    int            `gorm:"column:version"`
	SchemaJSON datatypes.JSON `gorm:"column:schema_json"`
	Enabled    bool           `gorm:"column:enabled"`
	CreatedBy  *string        `gorm:"column:created_by"`
	CreatedAt  time.Time      `gorm:"column:created_at"`
}

func (Contract) TableName() string { return "ops_event_contracts" }

type ConsumerOffset struct {
	ConsumerGroup    string    `gorm:"column:consumer_group;primaryKey"`
	LastSeenSequence int64     `gorm:"column:last_seen_sequence"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (ConsumerOffset) TableName() string { return "ops_consumer_offsets" }
