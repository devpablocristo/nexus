package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	"nexus-gateway/internal/org"
	"nexus-gateway/internal/shared/handlers"
)

func NewAuthMiddleware(l zerolog.Logger, auth org.AuthUsecase) gin.HandlerFunc {
	return handlers.AuthMiddleware(l, auth)
}

var MiddlewareSet = wire.NewSet(
	NewAuthMiddleware,
)
