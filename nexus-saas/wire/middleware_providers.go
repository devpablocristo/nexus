package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"nexus-saas/cmd/config"
	"nexus-saas/internal/identity"
	"nexus-saas/internal/org"
	"nexus-saas/internal/shared/handlers"
)

func NewAuthMiddleware(l zerolog.Logger, cfg config.ServiceConfig, auth *org.Usecases, jwtAuth *identity.Usecases) gin.HandlerFunc {
	return handlers.AuthMiddleware(l, cfg, auth, jwtAuth)
}

var MiddlewareSet = wire.NewSet(
	NewAuthMiddleware,
)
