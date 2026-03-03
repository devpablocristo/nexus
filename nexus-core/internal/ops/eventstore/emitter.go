package eventstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
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

type Emitter interface {
	Emit(ctx context.Context, in EmitInput) (opsdomain.StoredEvent, error)
}

type emitter struct {
	service *Usecases
}

func NewEmitter(service *Usecases) Emitter {
	return &emitter{service: service}
}

func (e *emitter) Emit(ctx context.Context, in EmitInput) (opsdomain.StoredEvent, error) {
	version := in.Version
	if version <= 0 {
		version = 1
	}
	occurred := in.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}
	payload := in.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return e.service.Append(ctx, opsdomain.Envelope{
		ID:          uuid.New(),
		EventType:   in.EventType,
		Version:     version,
		OccurredAt:  occurred,
		OrgID:       in.OrgID,
		Correlation: in.Correlation,
		Actor:       in.Actor,
		Source:      in.Source,
		Payload:     payload,
	})
}
