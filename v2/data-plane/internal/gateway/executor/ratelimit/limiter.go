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
	done    chan struct{}
}

type window struct {
	start time.Time
	count int
}

func NewInMemoryLimiter() *InMemoryLimiter {
	l := &InMemoryLimiter{
		windows: make(map[string]*window),
		now:     time.Now,
		done:    make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *InMemoryLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := l.now().UTC()
			l.mu.Lock()
			for k, w := range l.windows {
				if now.Sub(w.start) >= 2*time.Minute {
					delete(l.windows, k)
				}
			}
			l.mu.Unlock()
		case <-l.done:
			return
		}
	}
}

func (l *InMemoryLimiter) Close() {
	close(l.done)
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
