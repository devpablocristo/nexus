package wire

import (
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

func NewRateLimiter() *ratelimit.Limiter {
	return ratelimit.NewLimiter()
}

func NewHTTPExecutor(cfg config.ServiceConfig) *exechttp.Executor {
	return exechttp.NewExecutor(exechttp.Options{
		Timeout:          time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond,
		MaxResponseBytes: cfg.HTTPMaxResponseBytes,
		Retries:          1,
	})
}
