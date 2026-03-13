package dto

import (
	"time"

	"github.com/google/uuid"
	opsdomain "control-workers/internal/ops/eventstore/usecases/domain"
)

type EmitInput struct {
	EventType   string
	Version     int
	OccurredAt  time.Time
	OrgID       uuid.UUID
	Correlation opsdomain.Correlation
	Actor       opsdomain.Actor
	Source      string
	Payload     map[string]any
}
