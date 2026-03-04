package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"nexus-core/cmd/config"
	"nexus-core/internal/a2a"
	"nexus-core/internal/approval"
	"nexus-core/internal/audit"
	"nexus-core/internal/egress"
	"nexus-core/internal/gateway"
	"nexus-core/internal/identity"
	"nexus-core/internal/mcp"
	"nexus-core/internal/org"
	"nexus-core/internal/policy"
	"nexus-core/internal/secrets"
	"nexus-core/internal/tool"
	"nexus-core/internal/usagemetering"
	ginmw "nexus/pkg/http/middlewares/gin"
	ginserver "nexus/pkg/http/servers/gin"
)

func NewRouter(
	db *gorm.DB,
	l zerolog.Logger,
	cfg config.ServiceConfig,
	httpCfg config.HTTPServerConfig,
	authMw gin.HandlerFunc,
	toolH *tool.Handler,
	policyH *policy.Handler,
	auditH *audit.Handler,
	gwH *gateway.Handler,
	secretH *secrets.Handler,
	egressH *egress.Handler,
	mcpH *mcp.Handler,
	a2aH *a2a.Handler,
	oidcH *identity.OIDCHandler,
	approvalH *approval.Handler,
	orgH *org.Handler,
	usageMeteringMw usagemetering.APICallsMiddlewareFunc,
) *gin.Engine {
	r := ginserver.NewEngine(
		ginserver.EngineOptions{},
		ginmw.RequestID(),
		ginmw.Recovery(l),
		ginmw.CORS(cfg.CORSAllowedOrigins, cfg.CORSAllowedMethods, cfg.CORSAllowedHeaders),
		ginmw.BodyLimit(httpCfg.MaxBodyBytes),
		ginmw.LoggerMiddleware(l),
	)
	if cfg.OTelEnabled {
		r.Use(otelgin.Middleware(cfg.OTelServiceName))
	}
	prom := ginprometheus.NewPrometheus("nexus_gateway")
	prom.Use(r)

	registerHealthAndDocs(r, db, serviceConfigForRoutes{SwaggerCDN: cfg.SwaggerCDN})

	// OIDC endpoints are public (no auth middleware) because they are
	// the entry point for authentication itself.
	oidcGroup := r.Group("/v1")
	oidcH.Register(oidcGroup)

	onboardGroup := r.Group("/v1")
	orgH.Register(onboardGroup)

	v1 := r.Group("/v1")
	v1.Use(authMw)
	v1.Use(gin.HandlerFunc(usageMeteringMw))
	toolH.Register(v1)
	policyH.Register(v1)
	auditH.Register(v1)
	gwH.Register(v1)
	secretH.Register(v1)
	egressH.Register(v1)
	approvalH.Register(v1)

	mcpGroup := r.Group("")
	mcpGroup.Use(authMw)
	mcpH.Register(mcpGroup)

	a2aGroup := r.Group("")
	a2aGroup.Use(authMw)
	a2aH.Register(a2aGroup)

	return r
}

const swaggerHTMLCDN = `<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Nexus API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: '/openapi.yaml', dom_id: '#swagger-ui' });
  </script>
</body>
</html>`

const swaggerHTMLOfflineNote = `<!doctype html>
<html>
<head><meta charset="utf-8"/><title>Nexus API Docs</title></head>
<body>
  <h2>Swagger UI requires external assets</h2>
  <p>Set NEXUS_SWAGGER_CDN=true (default) to use the Swagger UI CDN bundles.</p>
  <p>The OpenAPI spec is always available at <code>/openapi.yaml</code>.</p>
</body>
</html>`
