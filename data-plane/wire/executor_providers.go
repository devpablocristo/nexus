package wire

import (
	"strings"
	"time"

	"github.com/google/wire"

	"data-plane/cmd/config"
	"data-plane/internal/gateway/executor/circuitbreaker"
	exechttp "data-plane/internal/gateway/executor/http"
	"data-plane/internal/gateway/executor/ratelimit"
	"nexus/pkg/utils"
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
	opts := exechttp.Options{
		Timeout:          time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond,
		MaxResponseBytes: cfg.HTTPMaxResponseBytes,
		Retries:          1,
		CircuitBreaker: circuitbreaker.Config{
			FailureThreshold: max(cfg.CBFailureThreshold, 1),
			HalfOpenMax:      max(cfg.CBHalfOpenMax, 1),
			ResetTimeout:     time.Duration(max(cfg.CBResetTimeoutSec, 5)) * time.Second,
		},
	}
	if !cfg.DisableSSRFProtection {
		opts.Transport = utils.SafeTransportWithAllowlist(cfg.EgressAllowlist)
		opts.CheckRedirect = utils.NoFollowRedirectPolicy
	}

	return exechttp.NewExecutor(opts)
}
