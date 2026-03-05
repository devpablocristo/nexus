package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"nexus-saas/cmd/config"
	"nexus-saas/internal/actions"
	"nexus-saas/internal/admin"
	"nexus-saas/internal/alerts"
	"nexus-saas/internal/assistant"
	"nexus-saas/internal/billing"
	"nexus-saas/internal/clerkwebhook"
	"nexus-saas/internal/contracts"
	"nexus-saas/internal/coreproxy"
	"nexus-saas/internal/events"
	"nexus-saas/internal/identity"
	"nexus-saas/internal/incidents"
	"nexus-saas/internal/notifications"
	"nexus-saas/internal/org"
	"nexus-saas/internal/policyproposal"
	"nexus-saas/internal/session"
	sharedratelimit "nexus-saas/internal/shared/ratelimit"
	"nexus-saas/internal/usagemetering"
	"nexus-saas/internal/users"
	ginmw "nexus/pkg/http/middlewares/gin"
	ginserver "nexus/pkg/http/servers/gin"
)

func NewRouter(
	db *gorm.DB,
	l zerolog.Logger,
	cfg config.ServiceConfig,
	httpCfg config.HTTPServerConfig,
	authMw gin.HandlerFunc,
	adminH *admin.Handler,
	billingH *billing.Handler,
	eventsH *events.Handler,
	actionsH *actions.Handler,
	incidentsH *incidents.Handler,
	notificationsH *notifications.Handler,
	alertsH *alerts.Handler,
	sessionH *session.Handler,
	proposalH *policyproposal.Handler,
	assistantH *assistant.Handler,
	oidcH *identity.OIDCHandler,
	orgH *org.Handler,
	usersH *users.Handler,
	clerkWebhookH *clerkwebhook.Handler,
	contractsH *contracts.Handler,
	coreProxyH *coreproxy.Handler,
	usageMeteringMw usagemetering.APICallsMiddlewareFunc,
) *gin.Engine {
	_ = db
	r := ginserver.NewEngine(
		ginserver.EngineOptions{},
		ginmw.RequestID(),
		ginmw.Recovery(l),
		ginmw.SecurityHeaders(),
		ginmw.CORS(cfg.CORSAllowedOrigins, cfg.CORSAllowedMethods, cfg.CORSAllowedHeaders),
		ginmw.BodyLimit(httpCfg.MaxBodyBytes),
	)
	if cfg.OTelEnabled {
		r.Use(otelgin.Middleware(cfg.OTelServiceName))
	}
	r.Use(ginmw.TraceContext())
	r.Use(ginmw.LoggerMiddleware(l))
	prom := ginprometheus.NewPrometheus("nexus_saas")
	prom.Use(r)

	registerHealthAndDocs(r, serviceConfigForRoutes{SwaggerCDN: cfg.SwaggerCDN})

	oidcGroup := r.Group("/v1")
	oidcH.Register(oidcGroup)

	webhookGroup := r.Group("/v1")
	clerkWebhookH.Register(webhookGroup)
	billingH.RegisterWebhook(r)

	onboardGroup := r.Group("/v1")
	orgH.Register(onboardGroup)

	contractsH.RegisterInternal(r)

	v1 := r.Group("/v1")
	v1.Use(authMw)
	tenantLimiter := sharedratelimit.NewTenantLimiter(cfg.SaaSRateLimitRPS, cfg.SaaSRateLimitBurst)
	v1.Use(tenantLimiter.Middleware())
	v1.Use(gin.HandlerFunc(usageMeteringMw))
	adminH.Register(v1)
	billingH.Register(v1)
	eventsH.Register(v1)
	actionsH.Register(v1)
	incidentsH.Register(v1)
	notificationsH.Register(v1)
	alertsH.Register(v1)
	sessionH.Register(v1)
	proposalH.Register(v1)
	assistantH.Register(v1)
	usersH.Register(v1)
	coreProxyH.Register(v1)

	return r
}
