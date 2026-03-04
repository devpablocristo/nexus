package eventstore

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"nexus-control-operators/internal/shared/coreerr"
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

const (
	backoffMin         = 1 * time.Second
	backoffMax         = 30 * time.Second
	maxHandlerRetries  = 3
)

type Consumer struct {
	service       consumerPort
	consumerGroup string
	batchSize     int
	pollInterval  time.Duration
	onIdle        func(ctx context.Context) error
	log           zerolog.Logger
}

func NewConsumer(service consumerPort, consumerGroup string, cfg consumerdto.ConsumerConfig, log zerolog.Logger) *Consumer {
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
		log:           log.With().Str("consumer", consumerGroup).Logger(),
	}
}

func (c *Consumer) Run(ctx context.Context, handler EventHandler) error {
	lastSeen, err := c.service.GetConsumerOffset(ctx, c.consumerGroup)
	if err != nil {
		return err
	}

	infraErrors := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		items, err := c.service.ListGlobalAfterSequence(ctx, lastSeen, c.batchSize)
		if err != nil {
			c.log.Warn().Err(err).Int("consecutive", infraErrors+1).Msg("list events failed, backing off")
			infraErrors++
			if waitErr := backoffWait(ctx, infraErrors); waitErr != nil {
				return nil
			}
			continue
		}
		infraErrors = 0

		if len(items) == 0 {
			if c.onIdle != nil {
				if idleErr := c.onIdle(ctx); idleErr != nil {
					c.log.Warn().Err(idleErr).Msg("onIdle failed")
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
			if processErr := c.processEvent(ctx, handler, ev); processErr != nil {
				c.log.Error().Err(processErr).Int64("seq", ev.Sequence).Msg("event permanently failed, skipping")
			}
			if ackErr := c.service.Ack(ctx, c.consumerGroup, ev.Sequence); ackErr != nil {
				c.log.Warn().Err(ackErr).Int64("seq", ev.Sequence).Msg("ack failed, retrying next cycle")
			}
			lastSeen = ev.Sequence
		}
	}
}

func (c *Consumer) processEvent(ctx context.Context, handler EventHandler, ev opsdomain.StoredEvent) error {
	for attempt := 1; attempt <= maxHandlerRetries; attempt++ {
		err := handler(ctx, ev)
		if err == nil {
			return nil
		}

		var coreErr *coreerr.CoreError
		retryable := errors.As(err, &coreErr) && coreErr.IsRetryable()
		if !retryable && !isTransient(err) {
			return err
		}

		c.log.Warn().Err(err).Int("attempt", attempt).Int64("seq", ev.Sequence).Msg("handler failed, retrying")
		if waitErr := backoffWait(ctx, attempt); waitErr != nil {
			return nil
		}
	}
	return errors.New("max retries exceeded")
}

func isTransient(err error) bool {
	var coreErr *coreerr.CoreError
	return errors.As(err, &coreErr) && coreErr.IsRetryable()
}

func backoffWait(ctx context.Context, attempt int) error {
	delay := backoffMin << uint(attempt-1)
	if delay > backoffMax {
		delay = backoffMax
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}
