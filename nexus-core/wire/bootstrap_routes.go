package wire

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/zsais/go-gin-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"nexus-core/cmd/config"
	"nexus-core/internal/a2a"
	"nexus-core/internal/actions"
	"nexus-core/internal/admin"
	"nexus-core/internal/assistant"
	"nexus-core/internal/audit"
	"nexus-core/internal/egress"
	"nexus-core/internal/events"
	"nexus-core/internal/gateway"
	"nexus-core/internal/identity"
	"nexus-core/internal/incidents"
	"nexus-core/internal/mcp"
	"nexus-core/internal/policy"
	"nexus-core/internal/policyproposal"
	"nexus-core/internal/secrets"
	"nexus-core/internal/tool"
	"nexus-core/internal/world"
	ginmw "nexus-core/pkg/http/middlewares/gin"
	ginserver "nexus-core/pkg/http/servers/gin"
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
	adminH *admin.Handler,
	eventsH *events.Handler,
	actionsH *actions.Handler,
	incidentsH *incidents.Handler,
	proposalH *policyproposal.Handler,
	assistantH *assistant.Handler,
	gwH *gateway.Handler,
	secretH *secrets.Handler,
	egressH *egress.Handler,
	mcpH *mcp.Handler,
	a2aH *a2a.Handler,
	oidcH *identity.OIDCHandler,
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

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false})
			return
		}
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("docs/openapi.yaml")
	})

	r.GET("/docs", func(c *gin.Context) {
		_ = l
		if cfg.SwaggerCDN {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, swaggerHTMLCDN)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerHTMLOfflineNote)
	})
	r.Static("/admin/assets", "docs/admin/assets")
	r.StaticFile("/admin", "docs/admin/index.html")

	// OIDC endpoints are public (no auth middleware) because they are
	// the entry point for authentication itself.
	oidcGroup := r.Group("/v1")
	oidcH.Register(oidcGroup)

	worldH := world.NewHandler(world.NewService(world.Config{
		BaseURL:     cfg.WorldSimBaseURL,
		InternalKey: cfg.WorldSimInternalKey,
		Timeout:     6 * time.Second,
	}))

	v1 := r.Group("/v1")
	v1.Use(authMw)
	toolH.Register(v1)
	policyH.Register(v1)
	auditH.Register(v1)
	adminH.Register(v1)
	eventsH.Register(v1)
	actionsH.Register(v1)
	incidentsH.Register(v1)
	proposalH.Register(v1)
	assistantH.Register(v1)
	gwH.Register(v1)
	secretH.Register(v1)
	egressH.Register(v1)
	worldH.Register(v1)

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
