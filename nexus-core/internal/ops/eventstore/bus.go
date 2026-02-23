package eventstore

import (
	"context"
	"time"

	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type EventHandler func(ctx context.Context, event opsdomain.StoredEvent) error

type ConsumerConfig struct {
	BatchSize    int
	PollInterval time.Duration
	OnIdle       func(ctx context.Context) error
}

type Consumer struct {
	service       Service
	consumerGroup string
	batchSize     int
	pollInterval  time.Duration
	onIdle        func(ctx context.Context) error
}

func NewConsumer(service Service, consumerGroup string, cfg ConsumerConfig) *Consumer {
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
