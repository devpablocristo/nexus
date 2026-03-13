package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed   State = iota // healthy, requests pass through
	StateOpen                  // failures exceeded threshold, requests rejected
	StateHalfOpen              // testing if upstream recovered
)

type Breaker struct {
	mu              sync.Mutex
	state           State
	failureCount    int
	successCount    int
	failureThreshold int
	halfOpenMax      int
	resetTimeout     time.Duration
	openedAt         time.Time
}

type Config struct {
	FailureThreshold int
	HalfOpenMax      int
	ResetTimeout     time.Duration
}

func DefaultConfig() Config {
	return Config{
		FailureThreshold: 5,
		HalfOpenMax:      2,
		ResetTimeout:     30 * time.Second,
	}
}

func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 2
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = 30 * time.Second
	}
	return &Breaker{
		state:            StateClosed,
		failureThreshold: cfg.FailureThreshold,
		halfOpenMax:      cfg.HalfOpenMax,
		resetTimeout:     cfg.ResetTimeout,
	}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(b.openedAt) >= b.resetTimeout {
			b.state = StateHalfOpen
			b.successCount = 0
			b.failureCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateHalfOpen:
		b.successCount++
		if b.successCount >= b.halfOpenMax {
			b.state = StateClosed
			b.failureCount = 0
			b.successCount = 0
		}
	case StateClosed:
		b.failureCount = 0
	}
}

func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		b.failureCount++
		if b.failureCount >= b.failureThreshold {
			b.state = StateOpen
			b.openedAt = time.Now()
		}
	case StateHalfOpen:
		b.state = StateOpen
		b.openedAt = time.Now()
		b.failureCount = 0
		b.successCount = 0
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen && time.Since(b.openedAt) >= b.resetTimeout {
		return StateHalfOpen
	}
	return b.state
}

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}
