package eventstore

import (
	"context"

	"github.com/google/uuid"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type RepositoryPort interface {
	Append(ctx context.Context, event opsdomain.Envelope, schemaValid bool, validationError *string) (opsdomain.StoredEvent, error)
	ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error)
	ListGlobalAfterSequence(ctx context.Context, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error)
	GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error)
	UpsertConsumerOffset(ctx context.Context, consumerGroup string, sequence int64) error
	UpsertContract(ctx context.Context, in opsdomain.EventContract) error
	GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error)
}

type ValidatorPort interface {
	ValidateEnvelope(ctx context.Context, event opsdomain.Envelope) error
	ValidatePayload(ctx context.Context, eventType string, version int, payload map[string]any) error
}

type Usecases struct {
	repo      RepositoryPort
	validator ValidatorPort
}

func NewUsecases(repo RepositoryPort, validator ValidatorPort) *Usecases {
	return &Usecases{
		repo:      repo,
		validator: validator,
	}
}

func (u *Usecases) Append(ctx context.Context, event opsdomain.Envelope) (opsdomain.StoredEvent, error) {
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	var validationError *string
	schemaValid := true
	if u.validator != nil {
		if err := u.validator.ValidateEnvelope(ctx, event); err != nil {
			schemaValid = false
			msg := err.Error()
			validationError = &msg
		} else if err := u.validator.ValidatePayload(ctx, event.EventType, event.Version, event.Payload); err != nil {
			schemaValid = false
			msg := err.Error()
			validationError = &msg
		}
	}
	return u.repo.Append(ctx, event, schemaValid, validationError)
}

func (u *Usecases) ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	return u.repo.ListAfterSequence(ctx, orgID, afterSequence, limit)
}

func (u *Usecases) ListGlobalAfterSequence(ctx context.Context, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	return u.repo.ListGlobalAfterSequence(ctx, afterSequence, limit)
}

func (u *Usecases) GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error) {
	return u.repo.GetConsumerOffset(ctx, consumerGroup)
}

func (u *Usecases) Ack(ctx context.Context, consumerGroup string, sequence int64) error {
	return u.repo.UpsertConsumerOffset(ctx, consumerGroup, sequence)
}

func (u *Usecases) UpsertContract(ctx context.Context, in opsdomain.EventContract) error {
	return u.repo.UpsertContract(ctx, in)
}

func (u *Usecases) GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error) {
	return u.repo.GetContract(ctx, eventType, version)
}
