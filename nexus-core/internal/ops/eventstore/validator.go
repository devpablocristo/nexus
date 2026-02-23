package eventstore

import (
	"context"

	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type noopValidator struct{}

func NewNoopValidator() ValidatorPort {
	return noopValidator{}
}

func (noopValidator) ValidateEnvelope(context.Context, opsdomain.Envelope) error {
	return nil
}

func (noopValidator) ValidatePayload(context.Context, string, int, map[string]any) error {
	return nil
}
