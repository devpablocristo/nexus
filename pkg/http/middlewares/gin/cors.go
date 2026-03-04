package ginmw

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS configures cross-origin requests using environment-provided allowlists.
// Keeping policy in config allows the same binary to run across all environments.
func CORS(allowedOriginsCSV, allowedMethodsCSV, allowedHeadersCSV string) gin.HandlerFunc {
	allowedOrigins := splitCSV(allowedOriginsCSV)
	if len(allowedOrigins) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	allowAllOrigins := contains(allowedOrigins, "*")
	allowedOriginSet := toSet(allowedOrigins)

	allowedMethods := strings.Join(splitCSV(allowedMethodsCSV), ", ")
	allowedHeaders := strings.Join(splitCSV(allowedHeadersCSV), ", ")

	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			c.Next()
			return
		}

		if !allowAllOrigins {
			if _, ok := allowedOriginSet[origin]; !ok {
				if c.Request.Method == http.MethodOptions {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
				c.Next()
				return
			}
		}

		c.Header("Vary", "Origin")
		if allowAllOrigins {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", allowedMethods)
		c.Header("Access-Control-Allow-Headers", allowedHeaders)
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
