package config

import "time"

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
	SaaSRateLimitRPS       float64
	SaaSRateLimitBurst     int

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

	// Internal service-to-service auth for Core -> SaaS contracts.
	SaaSInternalKey string

	// Clerk integration.
	// ClerkSecretKey is reserved for server-side Clerk Backend API calls
	// (e.g. fetching user details, managing organizations). Not yet used
	// but loaded from CLERK_SECRET_KEY for forward compatibility.
	ClerkSecretKey     string
	ClerkWebhookSecret string

	// Stripe Billing.
	StripeSecretKey       string
	StripeWebhookSecret   string
	StripePriceStarter    string
	StripePriceGrowth     string
	StripePriceEnterprise string
	TowerBaseURL          string

	// Notifications.
	NotificationBackend string
	AWSRegion           string
	SESFromEmail        string
	SESFromName         string
	SMTPHost            string
	SMTPPort            int
	SMTPFromEmail       string
	SMTPUsername        string
	SMTPPassword        string

	AlertEvalInterval time.Duration
}
