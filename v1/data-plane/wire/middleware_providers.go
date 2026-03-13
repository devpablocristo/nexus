package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"data-plane/cmd/config"
	"data-plane/internal/identity"
	"data-plane/internal/org"
	"data-plane/internal/shared/handlers"
)

func NewAuthMiddleware(l zerolog.Logger, cfg config.ServiceConfig, auth *org.Usecases, jwtAuth *identity.Usecases) gin.HandlerFunc {
	return handlers.AuthMiddleware(l, cfg, auth, jwtAuth)
}


var MiddlewareSet = wire.NewSet(
	NewAuthMiddleware,
)
