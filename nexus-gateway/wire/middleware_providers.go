package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"nexus-gateway/cmd/config"
	"nexus-gateway/internal/identity"
	"nexus-gateway/internal/org"
	"nexus-gateway/internal/shared/handlers"
)

func NewAuthMiddleware(l zerolog.Logger, cfg config.ServiceConfig, auth org.AuthUsecase, jwtAuth identity.Service) gin.HandlerFunc {
	return handlers.AuthMiddleware(l, cfg, auth, jwtAuth)
}

var MiddlewareSet = wire.NewSet(
	NewAuthMiddleware,
)
