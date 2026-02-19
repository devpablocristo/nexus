package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"nexus-gateway/pkg/config/godotenv"
)

type Config struct {
	API        APIConfig
	DB         DBConfig
	HTTPServer HTTPServerConfig
	Migrations MigrationsConfig
	Service    ServiceConfig
}

func Load() (Config, error) {
	_ = godotenv.LoadIfExists()

	var cfg Config

	port, err := mustInt("NEXUS_HTTP_PORT")
	if err != nil {
		return Config{}, err
	}
	cfg.API.HTTPPort = port

	cfg.DB.DatabaseURL = os.Getenv("NEXUS_DATABASE_URL")
	if cfg.DB.DatabaseURL == "" {
		return Config{}, errors.New("NEXUS_DATABASE_URL required")
	}

	timeoutMS, err := mustIntDefault("NEXUS_HTTP_TIMEOUT_MS", 5000)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.HTTPTimeoutMS = timeoutMS

	maxResp, err := mustInt64Default("NEXUS_HTTP_MAX_RESPONSE_BYTES", 1048576)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.HTTPMaxResponseBytes = maxResp

	maxBody, err := mustInt64Default("NEXUS_MAX_BODY_BYTES", 262144)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPServer.MaxBodyBytes = maxBody

	cfg.Service.LogLevel = mustStrDefault("NEXUS_LOG_LEVEL", "info")

	cfg.Service.SwaggerCDN = mustBoolDefault("NEXUS_SWAGGER_CDN", true)

	cfg.Service.MasterKey = os.Getenv("NEXUS_MASTER_KEY")
	if cfg.Service.MasterKey == "" {
		return Config{}, errors.New("NEXUS_MASTER_KEY required")
	}

	cfg.Service.AuthAllowAPIKey = mustBoolDefault("NEXUS_AUTH_ALLOW_API_KEY", true)
	cfg.Service.AuthEnableJWT = mustBoolDefault("NEXUS_AUTH_ENABLE_JWT", false)
	cfg.Service.JWKSURL = mustStrDefault("NEXUS_JWKS_URL", "")
	cfg.Service.JWTIssuer = mustStrDefault("NEXUS_JWT_ISSUER", "")
	cfg.Service.JWTAudience = mustStrDefault("NEXUS_JWT_AUDIENCE", "")
	cfg.Service.JWTOrgClaim = mustStrDefault("NEXUS_JWT_ORG_CLAIM", "org_id")
	cfg.Service.JWTRoleClaim = mustStrDefault("NEXUS_JWT_ROLE_CLAIM", "role")
	cfg.Service.JWTScopesClaim = mustStrDefault("NEXUS_JWT_SCOPES_CLAIM", "scopes")
	cfg.Service.JWTActorClaim = mustStrDefault("NEXUS_JWT_ACTOR_CLAIM", "sub")
	if cfg.Service.AuthEnableJWT && cfg.Service.JWKSURL == "" {
		return Config{}, errors.New("NEXUS_JWKS_URL required when NEXUS_AUTH_ENABLE_JWT=true")
	}

	rl, err := mustIntDefault("NEXUS_RATE_LIMIT_DEFAULT_PER_MINUTE", 60)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.RateLimitDefaultPerMin = rl
	cfg.Service.RateLimitBackend = mustStrDefault("NEXUS_RATE_LIMIT_BACKEND", "inmemory")
	cfg.Service.RedisURL = mustStrDefault("NEXUS_REDIS_URL", "redis://redis:6379/0")

	cfg.Service.OTelEnabled = mustBoolDefault("NEXUS_OTEL_ENABLED", false)
	cfg.Service.OTelServiceName = mustStrDefault("NEXUS_OTEL_SERVICE_NAME", "nexus-gateway")
	cfg.Service.OTLPEndpoint = mustStrDefault("NEXUS_OTLP_ENDPOINT", "")
	cfg.Service.OTLPInsecure = mustBoolDefault("NEXUS_OTLP_INSECURE", true)
	idt, err := mustIntDefault("NEXUS_IDEMPOTENCY_TTL_HOURS", 24)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.IdempotencyTTLHours = idt
	tbDef, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_DEFAULT_MS", 10000)
	if err != nil {
		return Config{}, err
	}
	tbMin, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_MIN_MS", 1000)
	if err != nil {
		return Config{}, err
	}
	tbMax, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_MAX_MS", 30000)
	if err != nil {
		return Config{}, err
	}
	cfg.Service.TimeoutBudgetDefaultMS = tbDef
	cfg.Service.TimeoutBudgetMinMS = tbMin
	cfg.Service.TimeoutBudgetMaxMS = tbMax

	cfg.Service.DisableSSRFProtection = mustBoolDefault("NEXUS_DISABLE_SSRF_PROTECTION", false)

	cfg.Migrations.Dir = mustStrDefault("NEXUS_MIGRATIONS_DIR", "./migrations")

	return cfg, nil
}

func mustStrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func mustInt(key string) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return 0, fmt.Errorf("%s required", key)
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int: %w", key, err)
	}
	return i, nil
}

func mustIntDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int: %w", key, err)
	}
	return i, nil
}

func mustInt64Default(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s invalid int64: %w", key, err)
	}
	return i, nil
}

func mustBoolDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
