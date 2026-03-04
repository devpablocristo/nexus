package runtime

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	opseventstore "nexus-control-operators/internal/ops/eventstore"
	opsdomain "nexus-control-operators/internal/ops/eventstore/usecases/domain"
)

type Worker interface {
	ConsumerGroup() string
	Handle(ctx context.Context, event opsdomain.StoredEvent) error
}

type IdleWorker interface {
	OnIdle(ctx context.Context) error
	IdleInterval() time.Duration
}

type Runner struct {
	consumer *opseventstore.Consumer
	worker   Worker
}

func NewRunner(eventService *opseventstore.Usecases, worker Worker, batchSize int, pollInterval time.Duration, log zerolog.Logger) *Runner {
	onIdle := func(context.Context) error { return nil }
	if iw, ok := worker.(IdleWorker); ok {
		lastTick := time.Time{}
		onIdle = func(ctx context.Context) error {
			now := time.Now().UTC()
			interval := iw.IdleInterval()
			if interval <= 0 {
				interval = 1 * time.Second
			}
			if !lastTick.IsZero() && now.Sub(lastTick) < interval {
				return nil
			}
			lastTick = now
			return iw.OnIdle(ctx)
		}
	}
	return &Runner{
		consumer: opseventstore.NewConsumer(eventService, worker.ConsumerGroup(), opseventstore.ConsumerConfig{
			BatchSize:    batchSize,
			PollInterval: pollInterval,
			OnIdle:       onIdle,
		}, log),
		worker: worker,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	return r.consumer.Run(ctx, r.worker.Handle)
}
