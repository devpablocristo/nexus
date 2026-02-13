package ginmw

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	httperr "nexus-gateway/pkg/http/errors"
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
				httperr.Write(c, 500, types.ErrCodeInternal, "internal error")
			}
		}()
		c.Next()
	}
}
