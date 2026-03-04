package eventstore

import (
	"context"
	"time"

	consumerdto "nexus-control-operators/internal/ops/eventstore/consumer/dto"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

// ConsumerConfig re-exported from consumer/dto for API stability.
type ConsumerConfig = consumerdto.ConsumerConfig

type EventHandler func(ctx context.Context, event opsdomain.StoredEvent) error

type consumerPort interface {
	ListGlobalAfterSequence(ctx context.Context, afterSequence int64, limit int) ([]opsdomain.StoredEvent, error)
	GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error)
	Ack(ctx context.Context, consumerGroup string, sequence int64) error
}

type Consumer struct {
	service       consumerPort
	consumerGroup string
	batchSize     int
	pollInterval  time.Duration
	onIdle        func(ctx context.Context) error
}

func NewConsumer(service consumerPort, consumerGroup string, cfg consumerdto.ConsumerConfig) *Consumer {
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 100
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = 700 * time.Millisecond
	}
	return &Consumer{
		service:       service,
		consumerGroup: consumerGroup,
		batchSize:     batch,
		pollInterval:  poll,
		onIdle:        cfg.OnIdle,
	}
}

func (c *Consumer) Run(ctx context.Context, handler EventHandler) error {
	lastSeen, err := c.service.GetConsumerOffset(ctx, c.consumerGroup)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		items, err := c.service.ListGlobalAfterSequence(ctx, lastSeen, c.batchSize)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			if c.onIdle != nil {
				if err := c.onIdle(ctx); err != nil {
					return err
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(c.pollInterval):
				continue
			}
		}

		for _, ev := range items {
			if err := handler(ctx, ev); err != nil {
				return err
			}
			if err := c.service.Ack(ctx, c.consumerGroup, ev.Sequence); err != nil {
				return err
			}
			lastSeen = ev.Sequence
		}
	}
}
