package ratelimit

import "testing"

func TestLimiterAllowPerMinuteWindow(t *testing.T) {
	t.Parallel()

	l := NewInMemoryLimiter()
	defer l.Close()

	if !l.Allow("k", 2) {
		t.Fatal("first call should be allowed")
	}
	if !l.Allow("k", 2) {
		t.Fatal("second call should be allowed")
	}
	if l.Allow("k", 2) {
		t.Fatal("third call should be denied")
	}
}
