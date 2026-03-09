// Package leasehttp verifies ephemeral execution lease tokens on HTTP boundaries.
package leasehttp

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"nexus/pkg/leaseauth"
)

const (
	ContextKeyClaims = "execution_lease_claims"
)

type Verifier struct {
	Issuer     string
	SigningKey string
}

func NewFromEnv() *Verifier {
	issuer := strings.TrimSpace(os.Getenv("NEXUS_EXECUTION_LEASE_TOKEN_ISSUER"))
	if issuer == "" {
		issuer = "nexus-core"
	}
	signingKey := strings.TrimSpace(os.Getenv("NEXUS_EXECUTION_LEASE_SIGNING_KEY"))
	if signingKey == "" {
		signingKey = strings.TrimSpace(os.Getenv("NEXUS_MASTER_KEY"))
	}
	return &Verifier{
		Issuer:     issuer,
		SigningKey: signingKey,
	}
}

func (v *Verifier) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		mode := strings.TrimSpace(c.GetHeader("X-Nexus-Credential-Mode"))
		token := TokenFromRequest(c.Request.Header)
		if mode == "" && token == "" {
			c.Next()
			return
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "execution lease token required"})
			return
		}
		if strings.TrimSpace(v.SigningKey) == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "execution lease verifier not configured"})
			return
		}
		claims, err := leaseauth.VerifyToken(v.SigningKey, token, v.Issuer, time.Now().UTC())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid execution lease token"})
			return
		}
		if mode != "" && !strings.EqualFold(mode, claims.CredentialMode) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "execution lease mode mismatch"})
			return
		}
		if header := strings.TrimSpace(c.GetHeader("X-Nexus-Lease-Id")); header != "" && header != claims.LeaseID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "execution lease id mismatch"})
			return
		}
		if header := strings.TrimSpace(c.GetHeader("X-Nexus-Intent-Id")); header != "" && header != claims.IntentID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "execution intent id mismatch"})
			return
		}
		if header := strings.TrimSpace(c.GetHeader("X-Nexus-Tool-Name")); header != "" && header != claims.ToolName {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "execution tool mismatch"})
			return
		}
		c.Set(ContextKeyClaims, claims)
		c.Next()
	}
}

func TokenFromRequest(headers http.Header) string {
	if token := strings.TrimSpace(headers.Get("X-Nexus-Execution-Token")); token != "" {
		return token
	}
	authz := strings.TrimSpace(headers.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return ""
	}
	return strings.TrimSpace(authz[7:])
}
