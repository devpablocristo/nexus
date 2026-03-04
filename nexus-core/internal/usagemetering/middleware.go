package usagemetering

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"nexus/pkg/types"
)

// APICallsMiddlewareFunc is a named type to avoid wire type collision with authMw gin.HandlerFunc.
type APICallsMiddlewareFunc gin.HandlerFunc

// NewAPICallsMiddleware returns a Gin middleware that increments the api_calls counter
// for the authenticated org. Fires in a goroutine after the handler to add zero latency.
func NewAPICallsMiddleware(m MeteringPort) APICallsMiddlewareFunc {
	return APICallsMiddlewareFunc(func(c *gin.Context) {
		c.Next()

		v, ok := c.Get(string(types.CtxKeyOrgID))
		if !ok {
			return
		}
		orgID, ok := v.(uuid.UUID)
		if !ok || orgID == uuid.Nil {
			return
		}
		go func(id uuid.UUID) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = m.Increment(ctx, id, CounterAPICalls)
		}(orgID)
	})
}
