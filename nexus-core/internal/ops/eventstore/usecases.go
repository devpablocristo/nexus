package eventstore

import (
	"context"

	"github.com/google/uuid"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type RepositoryPort interface {
	Append(ctx context.Context, event opsdomain.Envelope, schemaValid bool, validationError *string) (opsdomain.StoredEvent, error)
	ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error)
	GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error)
	UpsertConsumerOffset(ctx context.Context, consumerGroup string, sequence int64) error
	UpsertContract(ctx context.Context, in opsdomain.EventContract) error
	GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error)
}

type ValidatorPort interface {
	ValidateEnvelope(ctx context.Context, event opsdomain.Envelope) error
	ValidatePayload(ctx context.Context, eventType string, version int, payload map[string]any) error
}

type Service interface {
	Append(ctx context.Context, event opsdomain.Envelope) (opsdomain.StoredEvent, error)
	ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error)
	GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error)
	Ack(ctx context.Context, consumerGroup string, sequence int64) error
	UpsertContract(ctx context.Context, in opsdomain.EventContract) error
	GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error)
}

type service struct {
	repo      RepositoryPort
	validator ValidatorPort
}

func NewService(repo RepositoryPort, validator ValidatorPort) Service {
	return &service{
		repo:      repo,
		validator: validator,
	}
}

func (s *service) Append(ctx context.Context, event opsdomain.Envelope) (opsdomain.StoredEvent, error) {
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}
	var validationError *string
	schemaValid := true
	if s.validator != nil {
		if err := s.validator.ValidateEnvelope(ctx, event); err != nil {
			schemaValid = false
			msg := err.Error()
			validationError = &msg
		} else if err := s.validator.ValidatePayload(ctx, event.EventType, event.Version, event.Payload); err != nil {
			schemaValid = false
			msg := err.Error()
			validationError = &msg
		}
	}
	return s.repo.Append(ctx, event, schemaValid, validationError)
}

func (s *service) ListAfterSequence(ctx context.Context, orgID uuid.UUID, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error) {
	return s.repo.ListAfterSequence(ctx, orgID, afterSequence, limit)
}

func (s *service) GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error) {
	return s.repo.GetConsumerOffset(ctx, consumerGroup)
}

func (s *service) Ack(ctx context.Context, consumerGroup string, sequence int64) error {
	return s.repo.UpsertConsumerOffset(ctx, consumerGroup, sequence)
}

func (s *service) UpsertContract(ctx context.Context, in opsdomain.EventContract) error {
	return s.repo.UpsertContract(ctx, in)
}

func (s *service) GetContract(ctx context.Context, eventType string, version int) (opsdomain.EventContract, error) {
	return s.repo.GetContract(ctx, eventType, version)
}
