package ginmw

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"nexus/pkg/types"
)

// TraceContext extracts active OpenTelemetry trace/span IDs into gin context.
func TraceContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		sc := trace.SpanContextFromContext(c.Request.Context())
		if sc.HasTraceID() {
			c.Set(string(types.CtxKeyTraceID), sc.TraceID().String())
		}
		if sc.HasSpanID() {
			c.Set(string(types.CtxKeySpanID), sc.SpanID().String())
		}
		c.Next()
	}
}
