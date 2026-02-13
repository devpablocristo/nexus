package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/rs/zerolog"

	orguc "nexus-gateway/internal/org/usecases"
	"nexus-gateway/internal/shared/handlers"
)

func NewAuthMiddleware(l zerolog.Logger, auth orguc.AuthUsecase) gin.HandlerFunc {
	return handlers.AuthMiddleware(l, auth)
}

var MiddlewareSet = wire.NewSet(
	NewAuthMiddleware,
)
