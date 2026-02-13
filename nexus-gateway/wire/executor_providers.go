package wire

import (
	"strings"
	"time"

	"github.com/google/wire"

	"nexus-gateway/cmd/config"
	exechttp "nexus-gateway/internal/gateway/executor/http"
	"nexus-gateway/internal/gateway/executor/ratelimit"
)

var ExecutorSet = wire.NewSet(
	NewRateLimiter,
	NewHTTPExecutor,
)

func NewRateLimiter(cfg config.ServiceConfig) (ratelimit.Adapter, func(), error) {
	if strings.EqualFold(cfg.RateLimitBackend, "redis") {
		limiter, cleanup, err := ratelimit.NewRedisLimiter(cfg.RedisURL)
		if err != nil {
			return nil, nil, err
		}
		return limiter, cleanup, nil
	}
	return ratelimit.NewInMemoryLimiter(), func() {}, nil
}

func NewHTTPExecutor(cfg config.ServiceConfig) *exechttp.Executor {
	return exechttp.NewExecutor(exechttp.Options{
		Timeout:          time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond,
		MaxResponseBytes: cfg.HTTPMaxResponseBytes,
		Retries:          1,
	})
}
