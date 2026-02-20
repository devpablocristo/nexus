package wire

import (
	"github.com/google/wire"

	"nexus-core/cmd/config"
	"nexus-core/internal/identity"
	identityjwks "nexus-core/internal/identity/executor/jwks"
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

var IdentitySet = wire.NewSet(
	NewIdentityConfig,
	NewJWKSVerifier,
	wire.Bind(new(identity.TokenVerifierPort), new(*identityjwks.Verifier)),
	identity.NewService,
)
