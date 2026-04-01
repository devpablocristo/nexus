package wire

import (
	"context"
	"errors"
	"net/http"
	"strings"

	authn "github.com/devpablocristo/core/authn/go"
	authoidc "github.com/devpablocristo/core/authn/go/oidc"
	sharedapikey "github.com/devpablocristo/core/security/go/apikey"
)

func newAuthMiddleware(apiKeys, issuerURL, audience string) (func(http.Handler) http.Handler, error) {
	apiKeyAuth, err := newAPIKeyAuthenticator(apiKeys)
	if err != nil {
		return nil, err
	}
	jwtAuth := newJWTAuthenticator(issuerURL, audience)

	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.NotFoundHandler()
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			principal, method, err := authn.TryInbound(
				r.Context(),
				jwtAuth,
				apiKeyAuth,
				r.Header.Get("Authorization"),
				r.Header.Get("X-API-Key"),
			)
			if err != nil || principal == nil {
				writeUnauthorized(w)
				return
			}

			req := withIdentityHeaders(r, principal, method)
			next.ServeHTTP(w, req)
		})
	}, nil
}

func newAPIKeyAuthenticator(raw string) (authn.Authenticator, error) {
	base, err := sharedapikey.NewAuthenticator(raw)
	if err != nil {
		return nil, err
	}
	return &authn.APIKeyFuncAuthenticator{
		Resolve: func(_ context.Context, rawKey string) (*authn.Principal, error) {
			principal, ok := base.Authenticate(rawKey)
			if !ok {
				return nil, errors.New("authn: invalid api key")
			}
			return &authn.Principal{
				Actor:      principal.Name,
				Role:       principal.Name,
				AuthMethod: "api_key",
			}, nil
		},
	}, nil
}

func newJWTAuthenticator(issuerURL, audience string) authn.Authenticator {
	expectedIssuer := normalizeIssuer(issuerURL)
	if expectedIssuer == "" {
		return nil
	}

	discovery := authoidc.NewDiscoveryClient(expectedIssuer)
	expectedAudience := strings.TrimSpace(audience)

	return &authn.BearerJWTAuthenticator{
		Verify: discovery,
		Map: func(_ context.Context, claims map[string]any) (authn.Principal, error) {
			if normalizeIssuer(claims["iss"]) != expectedIssuer {
				return authn.Principal{}, errors.New("authn: invalid issuer")
			}
			if expectedAudience != "" &&
				!claimContainsAudience(claims["aud"], expectedAudience) &&
				!claimContainsAudience(claims["azp"], expectedAudience) {
				return authn.Principal{}, errors.New("authn: invalid audience")
			}

			sub := strings.TrimSpace(claimString(claims["sub"]))
			if sub == "" {
				return authn.Principal{}, errors.New("authn: missing sub claim")
			}

			actor := firstNonEmptyClaim(claims, "email", "preferred_username", "username", "sub")
			return authn.Principal{
				OrgID:      firstNonEmptyClaim(claims, "org_id", "tenant_id", "orgId"),
				Actor:      actor,
				Role:       firstNonEmptyClaim(claims, "role"),
				Scopes:     claimScopes(claims),
				Claims:     claims,
				AuthMethod: "jwt",
			}, nil
		},
	}
}

func withIdentityHeaders(r *http.Request, principal *authn.Principal, method string) *http.Request {
	if principal == nil || method != "jwt" {
		return r
	}

	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	if strings.TrimSpace(req.Header.Get("X-User-ID")) == "" && strings.TrimSpace(principal.Actor) != "" {
		req.Header.Set("X-User-ID", principal.Actor)
	}
	if strings.TrimSpace(req.Header.Get("X-Org-ID")) == "" && strings.TrimSpace(principal.OrgID) != "" {
		req.Header.Set("X-Org-ID", principal.OrgID)
	}
	return req
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"valid credentials required"}}`))
}

func normalizeIssuer(value any) string {
	return strings.TrimRight(strings.TrimSpace(claimString(value)), "/")
}

func claimString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}

func firstNonEmptyClaim(claims map[string]any, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(claimString(claims[name])); value != "" {
			return value
		}
	}
	return ""
}

func claimContainsAudience(value any, expected string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == expected
	case []string:
		for _, item := range v {
			if strings.TrimSpace(item) == expected {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if strings.TrimSpace(claimString(item)) == expected {
				return true
			}
		}
	}
	return false
}

func claimScopes(claims map[string]any) []string {
	raw := claims["scope"]
	if raw == nil {
		raw = claims["scp"]
	}

	switch v := raw.(type) {
	case string:
		parts := strings.Fields(v)
		return append([]string(nil), parts...)
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if scope := strings.TrimSpace(claimString(item)); scope != "" {
				out = append(out, scope)
			}
		}
		return out
	default:
		return nil
	}
}
