package wire

import (
	"strings"

	"github.com/google/wire"

	"data-plane/cmd/config"
	"data-plane/internal/identity"
	identityjwks "data-plane/internal/identity/executor/jwks"
	identityoidc "data-plane/internal/identity/executor/oidc"
)

func NewIdentityConfig(cfg config.ServiceConfig) identity.Config {
	return identity.Config{
		Issuer:      cfg.JWTIssuer,
		Audience:    cfg.JWTAudience,
		OrgClaim:    cfg.JWTOrgClaim,
		RoleClaim:   cfg.JWTRoleClaim,
		ScopesClaim: cfg.JWTScopesClaim,
		ActorClaim:  cfg.JWTActorClaim,
	}
}

func NewJWKSVerifier(cfg config.ServiceConfig) *identityjwks.Verifier {
	return identityjwks.NewVerifier(cfg.JWKSURL)
}

func NewOIDCConfig(cfg config.ServiceConfig) identity.OIDCConfig {
	scopes := parseOIDCScopesWire(cfg.OIDCScopes)
	return identity.OIDCConfig{
		Enabled:      cfg.OIDCEnabled,
		IssuerURL:    cfg.OIDCIssuerURL,
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Scopes:       scopes,
	}
}

func NewOIDCDiscoveryClient(cfg identity.OIDCConfig) *identityoidc.DiscoveryClient {
	if !cfg.Enabled {
		// Return a client with an empty issuer; it won't be used when OIDC is disabled.
		return identityoidc.NewDiscoveryClient("")
	}
	return identityoidc.NewDiscoveryClient(cfg.IssuerURL)
}

func NewOIDCTokenExchanger(cfg identity.OIDCConfig, discovery *identityoidc.DiscoveryClient) *identityoidc.TokenExchanger {
	if !cfg.Enabled {
		return identityoidc.NewTokenExchanger(discovery, "", "", "")
	}
	return identityoidc.NewTokenExchanger(discovery, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
}

func NewOIDCHandler(cfg identity.OIDCConfig, discovery *identityoidc.DiscoveryClient, exchanger *identityoidc.TokenExchanger, idSvc *identity.Usecases) *identity.OIDCHandler {
	return identity.NewOIDCHandler(cfg, discovery, exchanger, idSvc)
}

func parseOIDCScopesWire(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"openid", "profile", "email"}
	}
	raw = strings.ReplaceAll(raw, ",", " ")
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return []string{"openid", "profile", "email"}
	}
	return parts
}

var IdentitySet = wire.NewSet(
	NewIdentityConfig,
	NewJWKSVerifier,
	wire.Bind(new(identity.TokenVerifierPort), new(*identityjwks.Verifier)),
	identity.NewUsecases,
	NewOIDCConfig,
	NewOIDCDiscoveryClient,
	NewOIDCTokenExchanger,
	NewOIDCHandler,
)
