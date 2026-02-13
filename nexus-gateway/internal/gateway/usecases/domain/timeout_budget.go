package domain

import "time"

type TimeoutBudget struct {
	timeoutMS int
	start     time.Time
	stages    map[string]int64
}

func NewTimeoutBudget(timeoutMS int) *TimeoutBudget {
	if timeoutMS <= 0 {
		timeoutMS = 10000
	}
	return &TimeoutBudget{
		timeoutMS: timeoutMS,
		start:     time.Now(),
		stages:    map[string]int64{},
	}
}

func (b *TimeoutBudget) TimeoutMS() int {
	return b.timeoutMS
}

func (b *TimeoutBudget) RemainingMS() int {
	elapsed := time.Since(b.start).Milliseconds()
	remaining := int64(b.timeoutMS) - elapsed
	if remaining < 0 {
		return 0
	}
	return int(remaining)
}

func (b *TimeoutBudget) Consume(stage string, d time.Duration) {
	if b == nil || stage == "" {
		return
	}
	b.stages[stage] += d.Milliseconds()
}

func (b *TimeoutBudget) StageDurationsMS() map[string]int64 {
	out := make(map[string]int64, len(b.stages))
	for k, v := range b.stages {
		out[k] = v
	}
	return out
}

func ClampTimeoutMS(requested, def, min, max int) int {
	if min <= 0 {
		min = 1000
	}
	if max < min {
		max = min
	}
	val := requested
	if val <= 0 {
		val = def
	}
	if val <= 0 {
		val = 10000
	}
	if val < min {
		val = min
	}
	if val > max {
		val = max
	}
	return val
}
