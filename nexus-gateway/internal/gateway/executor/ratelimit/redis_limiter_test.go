package ratelimit

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisLimiterAllow(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	limiter, cleanup, err := NewRedisLimiter("redis://" + mr.Addr() + "/0")
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	defer cleanup()

	if !limiter.Allow("org:tool", 2) {
		t.Fatalf("expected allow #1")
	}
	if !limiter.Allow("org:tool", 2) {
		t.Fatalf("expected allow #2")
	}
	if limiter.Allow("org:tool", 2) {
		t.Fatalf("expected deny #3")
	}
}
