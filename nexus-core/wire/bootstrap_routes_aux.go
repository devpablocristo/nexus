package wire

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// registerHealthAndDocs registra rutas de health, docs y admin.
func registerHealthAndDocs(r *gin.Engine, db *gorm.DB, cfg serviceConfigForRoutes) {
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/docs")
	})
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
}

type serviceConfigForRoutes struct {
	SwaggerCDN bool
}
