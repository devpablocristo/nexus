package wire

import (
	"strings"
	"time"

	"github.com/google/wire"

	"nexus-core/cmd/config"
	exechttp "nexus-core/internal/gateway/executor/http"
	"nexus-core/internal/gateway/executor/ratelimit"
	"nexus-core/pkg/utils"
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
	}
	if !cfg.DisableSSRFProtection {
		opts.Transport = utils.SafeTransportWithAllowlist(cfg.EgressAllowlist)
		opts.CheckRedirect = utils.NoFollowRedirectPolicy
	}

	return exechttp.NewExecutor(opts)
}
