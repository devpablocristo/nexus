package ginmw

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"nexus-core/pkg/types"
)

const requestIDHeader = "X-Request-Id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(requestIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(string(types.CtxKeyRequestID), rid)
		c.Writer.Header().Set(requestIDHeader, rid)
		c.Next()
	}
}

func RequestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(string(types.CtxKeyRequestID)); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func LoggerMiddleware(l zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := nowMillis()
		c.Next()
		lat := nowMillis() - start

		status := c.Writer.Status()
		ev := l.Info()
		if status >= http.StatusInternalServerError {
			ev = l.Error()
		} else if status >= http.StatusBadRequest {
			ev = l.Warn()
		}

		orgID, _ := c.Get(string(types.CtxKeyOrgID))
		ev = ev.Str("org_id", toString(orgID))

		ev.
			Str("request_id", RequestIDFromContext(c)).
			Str("method", c.Request.Method).
			Str("path", c.FullPath()).
			Int("status", status).
			Int64("latency_ms", lat).
			Msg("http_request")
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case interface{ String() string }:
		return t.String()
	default:
		return ""
	}
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}
