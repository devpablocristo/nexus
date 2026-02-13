package wire

import (
	"github.com/google/wire"

	"nexus-gateway/internal/secrets"
	"nexus-gateway/internal/tool"
)

func ProvideSecretsToolLookup(s tool.Service) secrets.ToolLookupPort { return s }

var SecretsSet = wire.NewSet(
	secrets.NewRepository,
	wire.Bind(new(secrets.RepositoryPort), new(*secrets.Repository)),
	ProvideSecretsToolLookup,
	secrets.NewService,
	secrets.NewHandler,
)
