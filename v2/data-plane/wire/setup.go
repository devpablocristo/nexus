package wire

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nexus/v2/data-plane/internal/approval"
	"nexus/v2/data-plane/internal/egress"
	"nexus/v2/data-plane/internal/gateway"
	httpexec "nexus/v2/data-plane/internal/gateway/executor/http"
	"nexus/v2/data-plane/internal/gateway/executor/ratelimit"
	"nexus/v2/data-plane/internal/policy"
	"nexus/v2/data-plane/internal/secrets"
	"nexus/v2/data-plane/internal/tool"
)

type Config struct {
	EchoURL          string
	HTTPTimeout      time.Duration
	RateLimitBackend string
	RedisURL         string
}

func NewServer(cfg Config) (http.Handler, func(), error) {
	tools := make([]tool.Definition, 0, 1)
	if cfg.EchoURL != "" {
		tools = append(tools, tool.Definition{
			ID:               "tool_echo",
			Name:             "echo",
			Kind:             tool.KindHTTP,
			Method:           http.MethodPost,
			URL:              cfg.EchoURL,
			Enabled:          true,
			InputSchemaJSON:  mustJSON(map[string]any{"type": "object", "required": []string{"hello"}, "properties": map[string]any{"hello": map[string]any{"type": "string"}}, "additionalProperties": true}),
			OutputSchemaJSON: mustJSON(map[string]any{"type": "object", "required": []string{"received"}, "properties": map[string]any{"received": map[string]any{"type": "object"}}, "additionalProperties": true}),
		})
	}

	repo := tool.NewInMemoryRepository(tools)
	policies := policy.NewInMemoryRepository(nil)
	idempotency := gateway.NewInMemoryIdempotencyRepository()
	secretRepo := secrets.NewInMemoryRepository(nil)
	limiter, cleanup, err := NewRateLimiter(cfg)
	if err != nil {
		return nil, nil, err
	}
	egressRules := make([]egress.Rule, 0, 1)
	if cfg.EchoURL != "" {
		if parsed, err := url.Parse(cfg.EchoURL); err == nil && parsed.Hostname() != "" {
			egressRules = append(egressRules, egress.Rule{
				ToolID:  "tool_echo",
				Host:    parsed.Hostname(),
				Enabled: true,
			})
		}
	}
	egressUC := egress.NewUsecases(egress.NewInMemoryRepository(egressRules))
	evaluator := policy.NewEvaluator()
	executor := httpexec.NewExecutor(cfg.HTTPTimeout)
	intentRepo := gateway.NewInMemoryIntentRepository()
	leaseRepo := gateway.NewInMemoryLeaseRepository()
	approvalRepo := approval.NewInMemoryRepository()
	approvalUC := approval.NewUsecases(approvalRepo).WithIntentPort(intentRepo)
	runUsecase := gateway.NewUsecases(repo, policies, idempotency, limiter, egressUC, secretRepo, evaluator, executor)
	runUsecase = runUsecase.WithIntentRepository(intentRepo).WithLeaseRepository(leaseRepo).WithApproval(approval.NewGatewayAdapter(approvalUC))
	policyUsecase := policy.NewUsecases(policies, repo, evaluator)

	mux := http.NewServeMux()
	gateway.NewHandler(runUsecase).Register(mux)
	policy.NewHandler(policyUsecase).Register(mux)
	approval.NewHandler(approvalUC).Register(mux)
	return mux, cleanup, nil
}

func NewRateLimiter(cfg Config) (ratelimit.Adapter, func(), error) {
	if strings.EqualFold(strings.TrimSpace(cfg.RateLimitBackend), "redis") {
		if strings.TrimSpace(cfg.RedisURL) == "" {
			return nil, nil, fmt.Errorf("redis url required when rate limit backend is redis")
		}
		return ratelimit.NewRedisLimiter(cfg.RedisURL)
	}

	limiter := ratelimit.NewInMemoryLimiter()
	return limiter, limiter.Close, nil
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
