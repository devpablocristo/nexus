package ratelimit

import (
	"strings"
	"sync"
	"time"
)

type failRecord struct {
	count     int
	firstAt   time.Time
	lockUntil time.Time
}

// AuthLimiter tracks auth failures per client IP and applies temporary lockout.
type AuthLimiter struct {
	mu       sync.Mutex
	attempts map[string]*failRecord
	maxFails int
	window   time.Duration
	lockout  time.Duration
	now      func() time.Time
	stopCh   chan struct{}
}

// NewAuthLimiter returns an in-memory auth limiter suitable for brute-force protection.
func NewAuthLimiter(maxFails int, window, lockout time.Duration) *AuthLimiter {
	if maxFails <= 0 {
		maxFails = 10
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	if lockout <= 0 {
		lockout = 15 * time.Minute
	}
	al := &AuthLimiter{
		attempts: make(map[string]*failRecord),
		maxFails: maxFails,
		window:   window,
		lockout:  lockout,
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
	go al.cleanup()
	return al
}

// Close stops background cleanup.
func (al *AuthLimiter) Close() {
	close(al.stopCh)
}

// IsBlocked reports whether the IP is currently blocked.
func (al *AuthLimiter) IsBlocked(ip string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	now := al.now()
	al.mu.Lock()
	defer al.mu.Unlock()
	rec, ok := al.attempts[ip]
	if !ok {
		return false
	}
	if now.Before(rec.lockUntil) {
		return true
	}
	if now.Sub(rec.firstAt) > al.window {
		delete(al.attempts, ip)
		return false
	}
	return false
}

// RecordFailure increments failed auth attempts for an IP.
func (al *AuthLimiter) RecordFailure(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	now := al.now()
	al.mu.Lock()
	defer al.mu.Unlock()

	rec, ok := al.attempts[ip]
	if !ok || now.Sub(rec.firstAt) > al.window {
		al.attempts[ip] = &failRecord{
			count:     1,
			firstAt:   now,
			lockUntil: time.Time{},
		}
		return
	}

	rec.count++
	if rec.count >= al.maxFails {
		rec.lockUntil = now.Add(al.lockout)
	}
}

// RecordSuccess clears failures for an IP.
func (al *AuthLimiter) RecordSuccess(ip string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	al.mu.Lock()
	defer al.mu.Unlock()
	delete(al.attempts, ip)
}

func (al *AuthLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := al.now()
			al.mu.Lock()
			for ip, rec := range al.attempts {
				if now.After(rec.lockUntil) && now.Sub(rec.firstAt) > al.window {
					delete(al.attempts, ip)
				}
			}
			al.mu.Unlock()
		case <-al.stopCh:
			return
		}
	}
}
