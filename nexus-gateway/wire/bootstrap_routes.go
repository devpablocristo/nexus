package wire

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"nexus-gateway/cmd/config"
	audithandler "nexus-gateway/internal/audit/handler"
	gwhandler "nexus-gateway/internal/gateway/handler"
	policyhandler "nexus-gateway/internal/policy/handler"
	toolhandler "nexus-gateway/internal/tool/handler"
	ginmw "nexus-gateway/pkg/http/middlewares/gin"
	ginserver "nexus-gateway/pkg/http/servers/gin"
)

func NewRouter(
	db *gorm.DB,
	l zerolog.Logger,
	cfg config.ServiceConfig,
	httpCfg config.HTTPServerConfig,
	authMw gin.HandlerFunc,
	toolH *toolhandler.Handler,
	policyH *policyhandler.Handler,
	auditH *audithandler.Handler,
	gwH *gwhandler.Handler,
) *gin.Engine {
	r := ginserver.NewEngine(ginserver.EngineOptions{}, ginmw.RequestID(), ginmw.Recovery(l), ginmw.BodyLimit(httpCfg.MaxBodyBytes), ginmw.LoggerMiddleware(l))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
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

	v1 := r.Group("/v1")
	v1.Use(authMw)
	toolH.Register(v1)
	policyH.Register(v1)
	auditH.Register(v1)
	gwH.Register(v1)

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
