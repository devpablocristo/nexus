package wire

import (
	"testing"

	"github.com/alicebob/miniredis/v2"

	"nexus/v2/data-plane/internal/gateway/executor/ratelimit"
)

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	t.Run("defaults to in memory", func(t *testing.T) {
		t.Parallel()

		limiter, cleanup, err := NewRateLimiter(Config{})
		if err != nil {
			t.Fatalf("NewRateLimiter returned error: %v", err)
		}
		defer cleanup()

		if _, ok := limiter.(*ratelimit.InMemoryLimiter); !ok {
			t.Fatalf("unexpected limiter type: %T", limiter)
		}
	})

	t.Run("builds redis limiter", func(t *testing.T) {
		t.Parallel()

		mr, err := miniredis.Run()
		if err != nil {
			t.Fatalf("start miniredis: %v", err)
		}
		defer mr.Close()

		limiter, cleanup, err := NewRateLimiter(Config{
			RateLimitBackend: "redis",
			RedisURL:         "redis://" + mr.Addr() + "/0",
		})
		if err != nil {
			t.Fatalf("NewRateLimiter returned error: %v", err)
		}
		defer cleanup()

		if _, ok := limiter.(*ratelimit.RedisLimiter); !ok {
			t.Fatalf("unexpected limiter type: %T", limiter)
		}
	})
}
