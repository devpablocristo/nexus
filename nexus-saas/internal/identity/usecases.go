package identity

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	identitydomain "nexus-saas/internal/identity/usecases/domain"
	"nexus/pkg/types"
)

type TokenVerifierPort interface {
	VerifyToken(ctx context.Context, token string) (map[string]any, error)
}

type Config struct {
	Issuer      string
	Audience    string
	OrgClaim    string
	RoleClaim   string
	ScopesClaim string
	ActorClaim  string
}

type Usecases struct {
	verifier TokenVerifierPort
	cfg      Config
}

func NewUsecases(verifier TokenVerifierPort, cfg Config) *Usecases {
	return &Usecases{
		verifier: verifier,
		cfg:      cfg,
	}
}

func (u *Usecases) ResolvePrincipal(ctx context.Context, bearerToken string) (identitydomain.Principal, error) {
	claims, err := u.verifier.VerifyToken(ctx, bearerToken)
	if err != nil {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid bearer token")
	}

	if u.cfg.Issuer != "" && toString(claims["iss"]) != u.cfg.Issuer {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid token issuer")
	}
	if u.cfg.Audience != "" && !audienceMatches(claims["aud"], u.cfg.Audience) {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid token audience")
	}

	orgRaw, ok := claims[u.cfg.OrgClaim]
	if !ok || toString(orgRaw) == "" {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, fmt.Sprintf("missing %s claim", u.cfg.OrgClaim))
	}
	orgID, err := uuid.Parse(toString(orgRaw))
	if err != nil {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid org_id claim")
	}

	actorClaim := u.cfg.ActorClaim
	if actorClaim == "" {
		actorClaim = "sub"
	}
	actor := strings.TrimSpace(toString(claims[actorClaim]))
	role := strings.TrimSpace(toString(claims[u.cfg.RoleClaim]))
	scopes := parseScopes(claims[u.cfg.ScopesClaim])

	return identitydomain.Principal{
		OrgID:  orgID,
		Actor:  actor,
		Role:   role,
		Scopes: scopes,
	}, nil
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func audienceMatches(v any, audience string) bool {
	switch t := v.(type) {
	case string:
		return t == audience
	case []any:
		for _, item := range t {
			if toString(item) == audience {
				return true
			}
		}
	case []string:
		for _, item := range t {
			if item == audience {
				return true
			}
		}
	}
	return false
}

func parseScopes(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		chunks := strings.Fields(t)
		if len(chunks) == 0 {
			return nil
		}
		return chunks
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			scope := strings.TrimSpace(toString(item))
			if scope != "" {
				out = append(out, scope)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, item := range t {
			scope := strings.TrimSpace(item)
			if scope != "" {
				out = append(out, scope)
			}
		}
		return out
	default:
		return nil
	}
}
