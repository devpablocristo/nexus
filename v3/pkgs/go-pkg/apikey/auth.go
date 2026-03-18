package apikey

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	sharedhandlers "github.com/devpablocristo/nexus/v3/pkgs/go-pkg/handlers"
)

const (
	// HeaderName is the preferred header for API key authentication.
	HeaderName = "X-API-Key"
)

type contextKey string

const principalContextKey contextKey = "nexus.apikey.principal"

// Principal identifies the caller authenticated via API key.
type Principal struct {
	Name string
}

// Authenticator validates inbound API keys and exposes the authenticated principal.
type Authenticator struct {
	entries []entry
}

type entry struct {
	name string
	hash [sha256.Size]byte
}

var (
	// ErrMissingAPIKeys indicates the service was configured without any allowed API keys.
	ErrMissingAPIKeys = errors.New("api key configuration required")
	// ErrInvalidConfig indicates the raw API key configuration string is malformed.
	ErrInvalidConfig = errors.New("invalid api key configuration")
)

// NewAuthenticator parses a raw config string like "admin=secret,data-plane=secret2".
func NewAuthenticator(raw string) (*Authenticator, error) {
	parsed, err := parse(raw)
	if err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		return nil, ErrMissingAPIKeys
	}
	return &Authenticator{entries: parsed}, nil
}

// Middleware enforces API key auth on every endpoint except health probes.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		principal, ok := a.authenticate(r)
		if !ok {
			sharedhandlers.WriteJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{
					"code":    "UNAUTHORIZED",
					"message": "valid api key required",
				},
			})
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey, principal)
		*r = *r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// PrincipalFromContext returns the authenticated principal, if any.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	return principal, ok
}

// Apply sets the outbound API key header when the provided key is not empty.
func Apply(r *http.Request, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	r.Header.Set(HeaderName, key)
}

func (a *Authenticator) authenticate(r *http.Request) (Principal, bool) {
	if a == nil {
		return Principal{}, false
	}
	rawKey := extractAPIKey(r)
	if rawKey == "" {
		return Principal{}, false
	}
	sum := sha256.Sum256([]byte(rawKey))
	for _, item := range a.entries {
		if subtle.ConstantTimeCompare(sum[:], item.hash[:]) == 1 {
			return Principal{Name: item.name}, true
		}
	}
	return Principal{}, false
}

func extractAPIKey(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get(HeaderName)); value != "" {
		return value
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authz, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authz, prefix))
}

func parse(raw string) ([]entry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := splitParts(raw)
	items := make([]entry, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		piece := strings.TrimSpace(part)
		if piece == "" {
			continue
		}
		name, secret, ok := strings.Cut(piece, "=")
		if !ok {
			return nil, fmt.Errorf("%w: expected name=secret pair", ErrInvalidConfig)
		}
		name = strings.TrimSpace(name)
		secret = strings.TrimSpace(secret)
		if name == "" || secret == "" {
			return nil, fmt.Errorf("%w: empty name or secret", ErrInvalidConfig)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("%w: duplicate key name %q", ErrInvalidConfig, name)
		}
		seen[name] = struct{}{}
		sum := sha256.Sum256([]byte(secret))
		items = append(items, entry{name: name, hash: sum})
	}
	if len(items) == 0 {
		return nil, ErrMissingAPIKeys
	}
	return items, nil
}

func splitParts(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n'
	})
}

// DebugHash returns the SHA-256 hash of a key in hex form. It is intended for tests and diagnostics.
func DebugHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
