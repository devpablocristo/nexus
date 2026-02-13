package ginmw

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"nexus-gateway/pkg/types"
)

func Recovery(l zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				l.Error().
					Str("request_id", RequestIDFromContext(c)).
					Interface("panic", r).
					Msg("panic")
				c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
					RequestID: RequestIDFromContext(c),
					Error: types.APIError{
						Code:    types.ErrCodeInternal,
						Message: "internal error",
					},
				})
			}
		}()
		c.Next()
	}
}
