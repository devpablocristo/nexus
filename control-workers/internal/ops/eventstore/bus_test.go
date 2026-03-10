package eventstore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	opsdomain "control-workers/internal/ops/eventstore/usecases/domain"
)

func TestConsumer_AckAdvancesOffset(t *testing.T) {
	t.Parallel()

	mock := &mockEventService{
		events: []opsdomain.StoredEvent{
			{Sequence: 1, Envelope: opsdomain.Envelope{ID: uuid.New()}},
			{Sequence: 2, Envelope: opsdomain.Envelope{ID: uuid.New()}},
		},
	}
	dlqPath := filepath.Join(t.TempDir(), "dead_letters.jsonl")
	consumer := NewConsumer(mock, "workers.sentry", ConsumerConfig{
		BatchSize:    10,
		PollInterval: 1 * time.Millisecond,
		DLQPath:      dlqPath,
	}, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handled := 0
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Run(ctx, func(_ context.Context, _ opsdomain.StoredEvent) error {
			handled++
			if handled >= 2 {
				cancel()
			}
			return nil
		})
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected consumer error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("consumer timeout")
	}

	if got := mock.lastAck(); got != 2 {
		t.Fatalf("unexpected ack sequence: got=%d want=2", got)
	}
}

func TestConsumer_SkipsNonRetryableHandlerErrorAndContinues(t *testing.T) {
	t.Parallel()

	mock := &mockEventService{
		events: []opsdomain.StoredEvent{
			{Sequence: 10, Envelope: opsdomain.Envelope{ID: uuid.New()}},
			{Sequence: 11, Envelope: opsdomain.Envelope{ID: uuid.New()}},
		},
	}
	dlqPath := filepath.Join(t.TempDir(), "dead_letters.jsonl")
	consumer := NewConsumer(mock, "workers.coordinator", ConsumerConfig{
		BatchSize:    10,
		PollInterval: 1 * time.Millisecond,
		DLQPath:      dlqPath,
	}, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handled := 0
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Run(ctx, func(_ context.Context, ev opsdomain.StoredEvent) error {
			handled++
			if ev.Sequence == 11 {
				cancel()
				return errors.New("boom")
			}
			return nil
		})
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected consumer error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("consumer timeout")
	}

	if handled < 2 {
		t.Fatalf("expected both events handled, got=%d", handled)
	}
	if got := mock.lastAck(); got != 11 {
		t.Fatalf("unexpected ack sequence: got=%d want=11 (skipped error, still acked)", got)
	}
}

type mockEventService struct {
	mu         sync.Mutex
	events      []opsdomain.StoredEvent
	acked       int64
	offsetByGroup map[string]int64
}

func (m *mockEventService) Append(context.Context, opsdomain.Envelope) (opsdomain.StoredEvent, error) {
	return opsdomain.StoredEvent{}, errors.New("not implemented")
}

func (m *mockEventService) ListAfterSequence(context.Context, uuid.UUID, int64, int) ([]opsdomain.StoredEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockEventService) ListGlobalAfterSequence(_ context.Context, afterSequence int64, _ int) ([]opsdomain.StoredEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]opsdomain.StoredEvent, 0)
	for _, ev := range m.events {
		if ev.Sequence > afterSequence {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *mockEventService) GetConsumerOffset(_ context.Context, consumerGroup string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.offsetByGroup == nil {
		m.offsetByGroup = map[string]int64{}
	}
	return m.offsetByGroup[consumerGroup], nil
}

func (m *mockEventService) Ack(_ context.Context, consumerGroup string, sequence int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.offsetByGroup == nil {
		m.offsetByGroup = map[string]int64{}
	}
	m.offsetByGroup[consumerGroup] = sequence
	m.acked = sequence
	return nil
}

func (m *mockEventService) UpsertContract(context.Context, opsdomain.EventContract) error {
	return errors.New("not implemented")
}

func (m *mockEventService) GetContract(context.Context, string, int) (opsdomain.EventContract, error) {
	return opsdomain.EventContract{}, errors.New("not implemented")
}

func (m *mockEventService) lastAck() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acked
}
