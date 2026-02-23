package runtime

import (
	"context"
	"time"

	opseventstore "nexus-core/internal/ops/eventstore"
	opsdomain "nexus-core/internal/ops/eventstore/usecases/domain"
)

type Worker interface {
	ConsumerGroup() string
	Handle(ctx context.Context, event opsdomain.StoredEvent) error
}

type Runner struct {
	consumer *opseventstore.Consumer
	worker   Worker
}

func NewRunner(eventService opseventstore.Service, worker Worker, batchSize int, pollInterval time.Duration) *Runner {
	return &Runner{
		consumer: opseventstore.NewConsumer(eventService, worker.ConsumerGroup(), opseventstore.ConsumerConfig{
			BatchSize:    batchSize,
			PollInterval: pollInterval,
		}),
		worker: worker,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	return r.consumer.Run(ctx, r.worker.Handle)
}
