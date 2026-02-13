package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	orguc "nexus-gateway/internal/org/usecases"
	ginmw "nexus-gateway/pkg/http/middlewares/gin"
	"nexus-gateway/pkg/types"
	"nexus-gateway/pkg/utils"
)

const (
	HeaderAPIKey = "X-NEXUS-GATEWAY-KEY"
	HeaderActor  = "X-NEXUS-ACTOR"
)

func AuthMiddleware(l zerolog.Logger, auth orguc.AuthUsecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(HeaderAPIKey)
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
				RequestID: ginmw.RequestIDFromContext(c),
				Error:     types.APIError{Code: types.ErrCodeUnauthorized, Message: "missing api key"},
			})
			return
		}
		hash := utils.SHA256Hex(apiKey)

		orgID, err := auth.ResolveOrgID(c.Request.Context(), hash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
				RequestID: ginmw.RequestIDFromContext(c),
				Error:     types.APIError{Code: types.ErrCodeUnauthorized, Message: "invalid api key"},
			})
			return
		}

		c.Set(string(types.CtxKeyOrgID), orgID)
		actor := strings.TrimSpace(c.GetHeader(HeaderActor))
		if actor != "" {
			c.Set(string(types.CtxKeyActor), actor)
		}
		_ = l
		c.Next()
	}
}
