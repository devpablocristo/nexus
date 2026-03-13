package models

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	OrgID     uuid.UUID `gorm:"column:org_id;type:uuid;not null"`
	EventType string    `gorm:"column:event_type;not null"`
	Payload   []byte    `gorm:"column:payload_json;type:jsonb;not null"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (Event) TableName() string { return "operational_events" }
