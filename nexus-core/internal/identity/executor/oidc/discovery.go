package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	identityjwks "nexus-core/internal/identity/executor/jwks"
)

// DiscoveryDocument represents the OpenID Connect discovery document
// returned from the /.well-known/openid-configuration endpoint.
type DiscoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint,omitempty"`
	JWKSURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported []string `json:"response_types_supported,omitempty"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
}

// DiscoveryClient fetches and caches the OIDC discovery document from a
// given issuer URL. It also lazily initialises a JWKS verifier using the
// discovered jwks_uri.
type DiscoveryClient struct {
	issuerURL  string
	httpClient *http.Client
	cacheTTL   time.Duration

	mu       sync.RWMutex
	doc      *DiscoveryDocument
	cachedAt time.Time
	verifier *identityjwks.Verifier
}

// NewDiscoveryClient creates a DiscoveryClient for the given OIDC issuer URL.
// The issuer URL should NOT include the /.well-known suffix.
func NewDiscoveryClient(issuerURL string) *DiscoveryClient {
	return &DiscoveryClient{
		issuerURL:  issuerURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cacheTTL:   10 * time.Minute,
	}
}

// Discover fetches (or returns cached) the OIDC discovery document.
func (d *DiscoveryClient) Discover(ctx context.Context) (*DiscoveryDocument, error) {
	d.mu.RLock()
	if d.doc != nil && time.Since(d.cachedAt) < d.cacheTTL {
		doc := d.doc
		d.mu.RUnlock()
		return doc, nil
	}
	d.mu.RUnlock()

	return d.refresh(ctx)
}

// Verifier returns a JWKS verifier initialised with the discovered jwks_uri.
// The verifier is lazily created on first call and reused thereafter.
// If the discovery document is refreshed and the jwks_uri changes, the
// verifier is recreated.
func (d *DiscoveryClient) Verifier(ctx context.Context) (*identityjwks.Verifier, error) {
	doc, err := d.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}

	d.mu.RLock()
	v := d.verifier
	d.mu.RUnlock()

	if v != nil {
		return v, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	// Double-check after acquiring the write lock.
	if d.verifier != nil {
		return d.verifier, nil
	}
	d.verifier = identityjwks.NewVerifier(doc.JWKSURI)
	return d.verifier, nil
}

// VerifyToken satisfies the identity.TokenVerifierPort interface by delegating
// to the underlying JWKS verifier discovered via OIDC.
func (d *DiscoveryClient) VerifyToken(ctx context.Context, token string) (map[string]any, error) {
	v, err := d.Verifier(ctx)
	if err != nil {
		return nil, err
	}
	return v.VerifyToken(ctx, token)
}

func (d *DiscoveryClient) refresh(ctx context.Context) (*DiscoveryDocument, error) {
	wellKnown := d.issuerURL + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc discovery status %d from %s", resp.StatusCode, wellKnown)
	}

	var doc DiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("oidc discovery decode: %w", err)
	}

	if doc.Issuer == "" {
		return nil, fmt.Errorf("oidc discovery: missing issuer in document")
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("oidc discovery: missing jwks_uri in document")
	}
	if doc.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery: missing authorization_endpoint in document")
	}
	if doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery: missing token_endpoint in document")
	}

	d.mu.Lock()
	// If the JWKS URI changed, reset the verifier so it gets recreated.
	if d.doc != nil && d.doc.JWKSURI != doc.JWKSURI {
		d.verifier = nil
	}
	d.doc = &doc
	d.cachedAt = time.Now()
	d.mu.Unlock()

	return &doc, nil
}
