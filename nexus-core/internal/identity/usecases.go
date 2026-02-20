package identity

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	identitydomain "nexus-core/internal/identity/usecases/domain"
	"nexus-core/pkg/types"
)

type TokenVerifierPort interface {
	VerifyToken(ctx context.Context, token string) (map[string]any, error)
}

type Service interface {
	ResolvePrincipal(ctx context.Context, bearerToken string) (identitydomain.Principal, error)
}

type Config struct {
	Issuer      string
	Audience    string
	OrgClaim    string
	RoleClaim   string
	ScopesClaim string
	ActorClaim  string
}

type service struct {
	verifier TokenVerifierPort
	cfg      Config
}

func NewService(verifier TokenVerifierPort, cfg Config) Service {
	return &service{
		verifier: verifier,
		cfg:      cfg,
	}
}

func (s *service) ResolvePrincipal(ctx context.Context, bearerToken string) (identitydomain.Principal, error) {
	claims, err := s.verifier.VerifyToken(ctx, bearerToken)
	if err != nil {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid bearer token")
	}

	if s.cfg.Issuer != "" && toString(claims["iss"]) != s.cfg.Issuer {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid token issuer")
	}
	if s.cfg.Audience != "" && !audienceMatches(claims["aud"], s.cfg.Audience) {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid token audience")
	}

	orgRaw, ok := claims[s.cfg.OrgClaim]
	if !ok || toString(orgRaw) == "" {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, fmt.Sprintf("missing %s claim", s.cfg.OrgClaim))
	}
	orgID, err := uuid.Parse(toString(orgRaw))
	if err != nil {
		return identitydomain.Principal{}, types.NewHTTPError(401, types.ErrCodeUnauthorized, "invalid org_id claim")
	}

	actorClaim := s.cfg.ActorClaim
	if actorClaim == "" {
		actorClaim = "sub"
	}
	actor := strings.TrimSpace(toString(claims[actorClaim]))
	role := strings.TrimSpace(toString(claims[s.cfg.RoleClaim]))
	scopes := parseScopes(claims[s.cfg.ScopesClaim])

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
