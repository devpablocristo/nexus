package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_AllowPerMinuteWindow(t *testing.T) {
	l := NewInMemoryLimiter()
	now := time.Date(2026, 2, 13, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	if !l.Allow("k", 2) {
		t.Fatalf("expected allow #1")
	}
	if !l.Allow("k", 2) {
		t.Fatalf("expected allow #2")
	}
	if l.Allow("k", 2) {
		t.Fatalf("expected deny #3")
	}

	now = now.Add(61 * time.Second)
	if !l.Allow("k", 2) {
		t.Fatalf("expected allow after window")
	}
}
