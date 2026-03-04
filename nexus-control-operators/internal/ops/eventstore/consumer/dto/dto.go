package dto

import (
	"context"
	"time"
)

// ConsumerConfig holds configuration for the event consumer.
type ConsumerConfig struct {
	BatchSize    int
	PollInterval time.Duration
	OnIdle       func(ctx context.Context) error
}
