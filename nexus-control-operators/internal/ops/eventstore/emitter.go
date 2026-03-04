package eventstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	emitterdto "nexus-control-operators/internal/ops/eventstore/emitter/dto"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// EmitInput re-exported from emitter/dto for API stability.
type EmitInput = emitterdto.EmitInput

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
