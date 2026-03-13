package ratelimit

import (
	"testing"
	"time"
)

func TestAuthLimiter_BlocksAfterMaxFailures(t *testing.T) {
	now := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	limiter := NewAuthLimiter(3, 5*time.Minute, 15*time.Minute)
	t.Cleanup(limiter.Close)
	limiter.now = func() time.Time { return now }

	ip := "203.0.113.10"
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)
	if limiter.IsBlocked(ip) {
		t.Fatalf("ip should not be blocked before max failures")
	}
	limiter.RecordFailure(ip)
	if !limiter.IsBlocked(ip) {
		t.Fatalf("ip should be blocked after max failures")
	}
}

func TestAuthLimiter_ResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 3, 5, 10, 0, 0, 0, time.UTC)
	limiter := NewAuthLimiter(3, 5*time.Minute, 15*time.Minute)
	t.Cleanup(limiter.Close)
	limiter.now = func() time.Time { return now }

	ip := "203.0.113.11"
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)

	now = now.Add(6 * time.Minute)
	limiter.RecordFailure(ip)
	if limiter.IsBlocked(ip) {
		t.Fatalf("ip should not be blocked; failure window already reset")
	}
}

func TestAuthLimiter_RecordSuccessClearsFailures(t *testing.T) {
	limiter := NewAuthLimiter(3, 5*time.Minute, 15*time.Minute)
	t.Cleanup(limiter.Close)

	ip := "203.0.113.12"
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)
	limiter.RecordFailure(ip)
	if !limiter.IsBlocked(ip) {
		t.Fatalf("ip should be blocked before success")
	}
	limiter.RecordSuccess(ip)
	if limiter.IsBlocked(ip) {
		t.Fatalf("ip should be unblocked after success")
	}
}
