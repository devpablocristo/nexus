package ratelimit

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisLimiterAllow(t *testing.T) {
	t.Parallel()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()

	limiter, cleanup, err := NewRedisLimiter("redis://" + mr.Addr() + "/0")
	if err != nil {
		t.Fatalf("NewRedisLimiter returned error: %v", err)
	}
	defer cleanup()

	if !limiter.Allow("org:tool", 2) {
		t.Fatal("expected first request allowed")
	}
	if !limiter.Allow("org:tool", 2) {
		t.Fatal("expected second request allowed")
	}
	if limiter.Allow("org:tool", 2) {
		t.Fatal("expected third request denied")
	}
}
