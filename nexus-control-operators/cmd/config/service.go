package config

type ServiceConfig struct {
	LogLevel               string
	SwaggerCDN             bool
	HTTPTimeoutMS          int
	HTTPMaxResponseBytes   int64
	RateLimitDefaultPerMin int
	MasterKey              string
	AuthAllowAPIKey        bool
	AuthEnableJWT          bool
	JWKSURL                string
	JWTIssuer              string
	JWTAudience            string
	JWTOrgClaim            string
	JWTRoleClaim           string
	JWTScopesClaim         string
	JWTActorClaim          string
	RateLimitBackend       string
	RedisURL               string
	OTelEnabled            bool
	OTelServiceName        string
	OTLPEndpoint           string
	OTLPInsecure           bool
	IdempotencyTTLHours    int
	TimeoutBudgetDefaultMS int
	TimeoutBudgetMinMS     int
	TimeoutBudgetMaxMS     int
	DisableSSRFProtection  bool
	EgressAllowlist        string
	CORSAllowedOrigins     string
	CORSAllowedMethods     string
	CORSAllowedHeaders     string

	// Circuit breaker
	CBFailureThreshold int
	CBHalfOpenMax      int
	CBResetTimeoutSec  int

	// OIDC/SSO configuration
	OIDCEnabled      bool
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       string
}
