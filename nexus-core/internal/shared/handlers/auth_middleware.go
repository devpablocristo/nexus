package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"nexus-core/cmd/config"
	"nexus-core/internal/identity"
	"nexus-core/internal/org"
	httperr "nexus-core/pkg/http/errors"
	"nexus-core/pkg/types"
	"nexus-core/pkg/utils"
)

const (
	HeaderAPIKey = "X-NEXUS-CORE-KEY"
	HeaderActor  = "X-NEXUS-ACTOR"
	HeaderRole   = "X-NEXUS-ROLE"
	HeaderScopes = "X-NEXUS-SCOPES"
)

func AuthMiddleware(l zerolog.Logger, cfg config.ServiceConfig, auth org.AuthUsecase, idAuth identity.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.AuthEnableJWT {
			if token := bearerToken(c.GetHeader("Authorization")); token != "" {
				principal, err := idAuth.ResolvePrincipal(c.Request.Context(), token)
				if err != nil {
					httperr.Unauthorized(c, "invalid bearer token")
					return
				}
				c.Set(string(types.CtxKeyOrgID), principal.OrgID)
				if principal.Actor != "" {
					c.Set(string(types.CtxKeyActor), principal.Actor)
				}
				if principal.Role != "" {
					c.Set(string(types.CtxKeyRole), principal.Role)
				}
				scopes := principal.Scopes
				if hs := strings.TrimSpace(c.GetHeader(HeaderScopes)); hs != "" {
					scopes = intersectScopes(principal.Scopes, splitCSV(hs))
				}
				c.Set(string(types.CtxKeyScopes), scopes)
				c.Set(string(types.CtxKeyAuthMethod), "jwt")
				c.Next()
				return
			}
		}

		apiKeyEnabled := cfg.AuthAllowAPIKey || !cfg.AuthEnableJWT
		if !apiKeyEnabled {
			httperr.Write(c, http.StatusUnauthorized, types.ErrCodeUnauthorized, "api key auth disabled")
			return
		}

		apiKey := c.GetHeader(HeaderAPIKey)
		if apiKey == "" {
			httperr.Unauthorized(c, "missing api key")
			return
		}
		hash := utils.SHA256Hex(apiKey)

		principal, err := auth.ResolvePrincipal(c.Request.Context(), hash)
		if err != nil {
			httperr.Unauthorized(c, "invalid api key")
			return
		}

		c.Set(string(types.CtxKeyOrgID), principal.OrgID)
		actor := strings.TrimSpace(c.GetHeader(HeaderActor))
		if actor != "" {
			c.Set(string(types.CtxKeyActor), actor)
		}
		role := strings.TrimSpace(c.GetHeader(HeaderRole))
		if role != "" {
			c.Set(string(types.CtxKeyRole), role)
		}
		effectiveScopes := principal.Scopes
		if hs := strings.TrimSpace(c.GetHeader(HeaderScopes)); hs != "" {
			requested := splitCSV(hs)
			effectiveScopes = intersectScopes(principal.Scopes, requested)
		}
		c.Set(string(types.CtxKeyScopes), effectiveScopes)
		c.Set(string(types.CtxKeyAuthMethod), "api_key")
		_ = l
		c.Next()
	}
}

func bearerToken(h string) string {
	if h == "" {
		return ""
	}
	const pfx = "Bearer "
	if !strings.HasPrefix(h, pfx) {
		return ""
	}
	return strings.TrimSpace(h[len(pfx):])
}

func splitCSV(s string) []string {
	items := strings.Split(s, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func intersectScopes(base, requested []string) []string {
	if len(base) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(base))
	for _, s := range base {
		set[s] = struct{}{}
	}
	var out []string
	for _, s := range requested {
		if _, ok := set[s]; ok {
			out = append(out, s)
		}
	}
	return out
}
