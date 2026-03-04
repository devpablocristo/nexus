package config

import (
	"errors"
	"os"
)

// loadDB carga configuración de base de datos.
func loadDB(cfg *Config) error {
	cfg.DB.DatabaseURL = os.Getenv("NEXUS_SAAS_DATABASE_URL")
	if cfg.DB.DatabaseURL == "" {
		// Backward-compatible fallback for old environments.
		cfg.DB.DatabaseURL = os.Getenv("NEXUS_DATABASE_URL")
	}
	if cfg.DB.DatabaseURL == "" {
		return errors.New("NEXUS_SAAS_DATABASE_URL required")
	}
	return nil
}

// loadHTTP carga configuración HTTP (timeout, max body, etc.).
func loadHTTP(cfg *Config) error {
	timeoutMS, err := mustIntDefault("NEXUS_HTTP_TIMEOUT_MS", 5000)
	if err != nil {
		return err
	}
	cfg.Service.HTTPTimeoutMS = timeoutMS

	maxResp, err := mustInt64Default("NEXUS_HTTP_MAX_RESPONSE_BYTES", 1048576)
	if err != nil {
		return err
	}
	cfg.Service.HTTPMaxResponseBytes = maxResp

	maxBody, err := mustInt64Default("NEXUS_MAX_BODY_BYTES", 262144)
	if err != nil {
		return err
	}
	cfg.HTTPServer.MaxBodyBytes = maxBody
	return nil
}

// loadAuth carga configuración de autenticación (API key, JWT).
func loadAuth(cfg *Config) error {
	cfg.Service.MasterKey = os.Getenv("NEXUS_MASTER_KEY")
	if cfg.Service.MasterKey == "" {
		return errors.New("NEXUS_MASTER_KEY required")
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
		return errors.New("NEXUS_JWKS_URL required when NEXUS_AUTH_ENABLE_JWT=true")
	}
	return nil
}

// loadRateLimit carga configuración de rate limit.
func loadRateLimit(cfg *Config) error {
	rl, err := mustIntDefault("NEXUS_RATE_LIMIT_DEFAULT_PER_MINUTE", 60)
	if err != nil {
		return err
	}
	cfg.Service.RateLimitDefaultPerMin = rl
	cfg.Service.RateLimitBackend = mustStrDefault("NEXUS_RATE_LIMIT_BACKEND", "inmemory")
	cfg.Service.RedisURL = mustStrDefault("NEXUS_REDIS_URL", "redis://redis:6379/0")
	return nil
}

// loadOTel carga configuración OpenTelemetry.
func loadOTel(cfg *Config) error {
	cfg.Service.OTelEnabled = mustBoolDefault("NEXUS_OTEL_ENABLED", false)
	cfg.Service.OTelServiceName = mustStrDefault("NEXUS_OTEL_SERVICE_NAME", "nexus-core")
	cfg.Service.OTLPEndpoint = mustStrDefault("NEXUS_OTLP_ENDPOINT", "")
	cfg.Service.OTLPInsecure = mustBoolDefault("NEXUS_OTLP_INSECURE", true)
	return nil
}

// loadGateway carga configuración del gateway (idempotency, timeout budget, egress).
func loadGateway(cfg *Config) error {
	idt, err := mustIntDefault("NEXUS_IDEMPOTENCY_TTL_HOURS", 24)
	if err != nil {
		return err
	}
	cfg.Service.IdempotencyTTLHours = idt

	tbDef, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_DEFAULT_MS", 10000)
	if err != nil {
		return err
	}
	tbMin, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_MIN_MS", 1000)
	if err != nil {
		return err
	}
	tbMax, err := mustIntDefault("NEXUS_TIMEOUT_BUDGET_MAX_MS", 30000)
	if err != nil {
		return err
	}
	cfg.Service.TimeoutBudgetDefaultMS = tbDef
	cfg.Service.TimeoutBudgetMinMS = tbMin
	cfg.Service.TimeoutBudgetMaxMS = tbMax

	cfg.Service.DisableSSRFProtection = mustBoolDefault("NEXUS_DISABLE_SSRF_PROTECTION", false)
	cfg.Service.EgressAllowlist = mustStrDefault("NEXUS_EGRESS_ALLOWLIST", "")
	cfg.Service.CORSAllowedOrigins = mustStrDefault("NEXUS_CORS_ALLOWED_ORIGINS", "")
	cfg.Service.CORSAllowedMethods = mustStrDefault("NEXUS_CORS_ALLOWED_METHODS", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
	cfg.Service.CORSAllowedHeaders = mustStrDefault("NEXUS_CORS_ALLOWED_HEADERS", "Authorization,Content-Type,X-NEXUS-CORE-KEY,X-NEXUS-SCOPES,X-NEXUS-ACTOR,Idempotency-Key,X-Timeout-Ms")

	cbFail, err := mustIntDefault("NEXUS_CB_FAILURE_THRESHOLD", 5)
	if err != nil {
		return err
	}
	cbHalf, err := mustIntDefault("NEXUS_CB_HALF_OPEN_MAX", 2)
	if err != nil {
		return err
	}
	cbReset, err := mustIntDefault("NEXUS_CB_RESET_TIMEOUT_SEC", 30)
	if err != nil {
		return err
	}
	cfg.Service.CBFailureThreshold = cbFail
	cfg.Service.CBHalfOpenMax = cbHalf
	cfg.Service.CBResetTimeoutSec = cbReset
	return nil
}

// loadOIDC carga configuración OIDC/SSO.
func loadOIDC(cfg *Config) error {
	cfg.Service.OIDCEnabled = mustBoolDefault("NEXUS_OIDC_ENABLED", false)
	cfg.Service.OIDCIssuerURL = mustStrDefault("NEXUS_OIDC_ISSUER_URL", "")
	cfg.Service.OIDCClientID = mustStrDefault("NEXUS_OIDC_CLIENT_ID", "")
	cfg.Service.OIDCClientSecret = mustStrDefault("NEXUS_OIDC_CLIENT_SECRET", "")
	cfg.Service.OIDCRedirectURL = mustStrDefault("NEXUS_OIDC_REDIRECT_URL", "")
	cfg.Service.OIDCScopes = mustStrDefault("NEXUS_OIDC_SCOPES", "openid profile email")
	if cfg.Service.OIDCEnabled {
		if cfg.Service.OIDCIssuerURL == "" {
			return errors.New("NEXUS_OIDC_ISSUER_URL required when NEXUS_OIDC_ENABLED=true")
		}
		if cfg.Service.OIDCClientID == "" {
			return errors.New("NEXUS_OIDC_CLIENT_ID required when NEXUS_OIDC_ENABLED=true")
		}
		if cfg.Service.OIDCRedirectURL == "" {
			return errors.New("NEXUS_OIDC_REDIRECT_URL required when NEXUS_OIDC_ENABLED=true")
		}
	}
	cfg.Service.SaaSInternalKey = mustStrDefault("NEXUS_SAAS_INTERNAL_KEY", "")
	return nil
}
