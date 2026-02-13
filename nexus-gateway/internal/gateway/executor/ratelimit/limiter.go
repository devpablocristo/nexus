package ratelimit

import (
	"sync"
	"time"
)

type Adapter interface {
	Allow(key string, perMinute int) bool
}

type InMemoryLimiter struct {
	mu      sync.Mutex
	windows map[string]*window
	now     func() time.Time
}

type window struct {
	start time.Time
	count int
}

func NewInMemoryLimiter() *InMemoryLimiter {
	return &InMemoryLimiter{
		windows: make(map[string]*window),
		now:     time.Now,
	}
}

func (l *InMemoryLimiter) Allow(key string, perMinute int) bool {
	if perMinute <= 0 {
		return true
	}
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		l.windows[key] = &window{start: now, count: 1}
		return true
	}
	if w.count >= perMinute {
		return false
	}
	w.count++
	return true
}
