package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusValid   Status = "valid"
	StatusInvalid Status = "invalid"
)

type Report struct {
	ID              uuid.UUID
	OrgID           uuid.UUID
	IncidentID      *uuid.UUID
	Provider        string
	Model           string
	Status          Status
	Report          map[string]any
	ValidationError *string
	CreatedBy       *string
	CreatedAt       time.Time
}
